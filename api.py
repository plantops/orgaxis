"""API-first temporal org & workflow resolution demo (synthetic data only)."""
from __future__ import annotations
import hmac
import re
import os
import sqlite3
from contextlib import contextmanager
from datetime import date
from pathlib import Path
from typing import Annotated

from fastapi import Depends, FastAPI, HTTPException, Query
from fastapi.responses import HTMLResponse
from fastapi.security import APIKeyHeader
from pydantic import BaseModel, Field

from engine import (DomainError, blank_org, chain_for_person, connect,
                    manager_for_person, move_assignment, occupied_org,
                    reporting_change, workflow_resolve, direct_reports, org_changes,
                    tenant_info, companies, add_company)

# Deliberately block production deployments until real auth/tenant isolation exists.
if os.getenv('APP_ENV','demo') == 'production':
    raise RuntimeError('Demo-only API. Production requires tenant-bound OIDC/RBAC and security review.')

DEMO_KEY = os.getenv('DEMO_API_KEY','demo-local-key')
api_key_header = APIKeyHeader(name='X-API-Key',auto_error=False)
app = FastAPI(title='Temporal Workforce Engine - DEMO',version='0.1.0',
              description='Synthetic-data demo only. Not yet a production-grade SaaS or statutory rules engine.')


def auth(key: Annotated[str | None, Depends(api_key_header)]):
    if not key or not hmac.compare_digest(key,DEMO_KEY):
        raise HTTPException(status_code=401,detail='Invalid API key')
    return True


@contextmanager
def session():
    c=connect()
    try:
        yield c
    finally:
        c.close()


def call(fn,*args,**kwargs):
    try:
        with session() as db:
            return fn(db,*args,**kwargs)
    except DomainError as e:
        raise HTTPException(status_code=e.status,detail=e.message) from e
    except sqlite3.OperationalError as e:
        if 'no such table' in str(e) or 'unable to open' in str(e):
            raise HTTPException(status_code=503,detail='Database not seeded; run seed.py') from e
        raise


class CompanyCreate(BaseModel):
    code: str = Field(min_length=2,max_length=32,pattern=r'^[A-Za-z0-9-]+$')
    name: str = Field(min_length=2,max_length=200)
    jurisdiction: str = Field(default='VN',min_length=2,max_length=8)


class MoveRequest(BaseModel):
    person_id: str = Field(pattern=r'^EMP-\d{4}$')
    target_position_code: str = Field(min_length=2,max_length=64)
    effective_date: date
    reason: str = Field(default='transfer',max_length=64)


class ReportingRequest(BaseModel):
    position_code: str = Field(min_length=2,max_length=64)
    supervisor_position_code: str = Field(min_length=2,max_length=64)
    effective_date: date


@app.get('/health')
def health():
    return {'status':'ok','mode':'demo','synthetic_data':True}


@app.get('/v1/people/{person_id}/manager',dependencies=[Depends(auth)])
def get_manager(person_id: str, as_of: date = Query(default_factory=date.today)):
    return call(manager_for_person,person_id,as_of.isoformat())


@app.get('/v1/people/{person_id}/chain',dependencies=[Depends(auth)])
def get_chain(person_id: str, as_of: date = Query(default_factory=date.today),
              max_levels: int=Query(default=20,ge=1,le=50)):
    return call(chain_for_person,person_id,as_of.isoformat(),max_levels)


@app.get('/v1/org/blank',dependencies=[Depends(auth)])
def get_blank_org(as_of: date = Query(default_factory=date.today)):
    return call(blank_org,as_of.isoformat())


@app.get('/v1/org/occupied',dependencies=[Depends(auth)])
def get_occupied_org(as_of: date=Query(default_factory=date.today),
                     limit: int=Query(default=100,ge=1,le=1000),offset: int=Query(default=0,ge=0)):
    return call(occupied_org,as_of.isoformat(),limit,offset)


@app.get('/v1/workflow/resolve',dependencies=[Depends(auth)])
def get_workflow(subject_id: str, as_of: date=Query(default_factory=date.today),
                 policy: str='LINE_MANAGER', levels: int=Query(default=1,ge=1,le=10)):
    return call(workflow_resolve,subject_id,as_of.isoformat(),policy,levels)


@app.post('/v1/assignments/move',dependencies=[Depends(auth)])
def post_move(data: MoveRequest):
    return call(move_assignment,data.person_id,data.target_position_code,
                data.effective_date.isoformat(),data.reason)


@app.post('/v1/org/reporting/change',dependencies=[Depends(auth)])
def post_reporting(data: ReportingRequest):
    return call(reporting_change,data.position_code,data.supervisor_position_code,
                data.effective_date.isoformat())


@app.get('/v1/people/{person_id}/reports',dependencies=[Depends(auth)])
def get_reports(person_id: str, as_of: date=Query(default_factory=date.today),
                limit: int=Query(default=100,ge=1,le=1000),offset: int=Query(default=0,ge=0)):
    return call(direct_reports,person_id,as_of.isoformat(),limit,offset)


@app.get('/v1/org/changes',dependencies=[Depends(auth)])
def get_org_changes():
    return call(org_changes)


@app.get('/v1/tenant', dependencies=[Depends(auth)])
def get_tenant():
    return call(tenant_info)


@app.get('/v1/companies', dependencies=[Depends(auth)])
def get_companies():
    return call(companies)


@app.post('/v1/companies', dependencies=[Depends(auth)], status_code=201)
def post_company(data: CompanyCreate):
    return call(add_company,data.code,data.name,data.jurisdiction)


@app.get('/', response_class=HTMLResponse, include_in_schema=False)
def ui():
    return (Path(__file__).resolve().parent / 'web/index.html').read_text(encoding='utf-8')
