import pytest
from fastapi.testclient import TestClient
import engine
from seed import seed
from api import app

@pytest.fixture
def client(tmp_path,monkeypatch):
    path=tmp_path/'test.sqlite3'
    monkeypatch.setattr(engine,'DB_PATH',path)
    with engine.connect() as db:
        seed(db)
    with TestClient(app) as client:
        yield client

AUTH={'X-API-Key':'demo-local-key'}

def get(client,path,**params):
    r=client.get(path,headers=AUTH,params=params)
    assert r.status_code==200, r.text
    return r.json()

def test_data_counts(client):
    with engine.connect() as db:
        counts={name:db.execute(f'SELECT COUNT(*) FROM {name}').fetchone()[0]
                for name in ('person','hr_event','assignment')}
        org=db.execute("SELECT COUNT(*) FROM change_set WHERE domain='org_structure'").fetchone()[0]
    assert counts=={'person':1000,'hr_event':80,'assignment':1080}
    assert org==2

def test_blank_org_changes_at_dates(client):
    early=get(client,'/v1/org/blank',as_of='2026-06-30')
    later=get(client,'/v1/org/blank',as_of='2026-07-01')
    future=get(client,'/v1/org/blank',as_of='2027-01-01')
    parent=lambda result: [x['parent_id'] for x in result['org_edges'] if x['child_id']=='OU-WH'][0]
    assert parent(early)=='OU-P1'
    assert parent(later)=='OU-OPS'
    assert (early['position_count'],future['position_count'])==(15,17)
    assert 'POS-LOG-MGR' in [x['id'] for x in future['positions']]
    assert 'POS-LOG-MGR' not in [x['id'] for x in early['positions']]

def test_personnel_time_travel(client):
    jan=get(client,'/v1/people/EMP-0030/manager',as_of='2026-02-01')
    nov=get(client,'/v1/people/EMP-0030/manager',as_of='2026-11-18')
    assert jan['manager']['person_id']=='EMP-0009'
    assert nov['manager']['person_id']=='EMP-0010'
    assert nov['position_code']=='PACK-OP'

def test_manager_of_manager_changes_when_org_changes(client):
    early=get(client,'/v1/people/EMP-0005/manager',as_of='2026-06-30')
    late=get(client,'/v1/people/EMP-0005/manager',as_of='2026-07-01')
    assert early['manager']['person_id']=='EMP-0003'
    assert late['manager']['person_id']=='EMP-0002'

def test_workflow_resolution(client):
    result=get(client,'/v1/workflow/resolve',subject_id='EMP-0020',as_of='2026-11-18',levels=2)
    assert result['status']=='resolved'
    assert result['assignee_id']=='EMP-0005'
    assert [x['person_id'] for x in result['full_chain']]==['EMP-0009','EMP-0005','EMP-0002','EMP-0001']

def test_reports_change_after_transfer(client):
    prior=get(client,'/v1/people/EMP-0009/reports',as_of='2026-01-01')
    after=get(client,'/v1/people/EMP-0009/reports',as_of='2026-11-18')
    assert prior['total']-after['total']==80

def test_authorization(client):
    assert client.get('/v1/org/blank').status_code==401
    assert client.get('/v1/org/blank',headers={'X-API-Key':'bad'}).status_code==401

def test_mutation_move_and_history(client):
    r=client.post('/v1/assignments/move',headers=AUTH,json={
        'person_id':'EMP-0110','target_position_code':'PACK-OP','effective_date':'2026-12-01'})
    assert r.status_code==200,r.text
    early=get(client,'/v1/people/EMP-0110/manager',as_of='2026-11-30')
    late=get(client,'/v1/people/EMP-0110/manager',as_of='2026-12-01')
    assert early['manager']['person_id']=='EMP-0009'
    assert late['manager']['person_id']=='EMP-0010'

def test_reporting_cycle_rejected(client):
    bad=client.post('/v1/org/reporting/change',headers=AUTH,json={
        'position_code':'WH-MGR','supervisor_position_code':'WH-LEAD','effective_date':'2026-12-01'})
    assert bad.status_code==409
    actual=get(client,'/v1/people/EMP-0005/manager',as_of='2026-12-02')
    assert actual['manager']['person_id']=='EMP-0002'

def test_reporting_change_succeeds(client):
    r=client.post('/v1/org/reporting/change',headers=AUTH,json={
        'position_code':'PACK-MGR','supervisor_position_code':'OPS-MGR','effective_date':'2026-12-01'})
    assert r.status_code==200, r.text
    before=get(client,'/v1/people/EMP-0006/manager',as_of='2026-11-30')
    after=get(client,'/v1/people/EMP-0006/manager',as_of='2026-12-01')
    assert before['manager']['person_id']=='EMP-0003'
    assert after['manager']['person_id']=='EMP-0002'

def test_unknown_person_and_future_vacancy(client):
    assert client.get('/v1/people/EMP-9999/manager',headers=AUTH).status_code==404
    future=get(client,'/v1/org/blank',as_of='2027-01-01')
    assert any(x['code']=='LOG-MGR' for x in future['positions'])
    with engine.connect() as db:
        out=engine._manager_from_position(db,'POS-LOG-OP','2027-01-01')
    assert out['status']=='vacant_supervisor'

def test_tenant_and_company_create(client):
    t=get(client,'/v1/tenant')
    assert t['id']=='tenant-demo'
    c=get(client,'/v1/companies')
    assert len(c['items'])==3
    r=client.post('/v1/companies',headers=AUTH,json={
        'code':'NEWCO','name':'New Synthetic Company','jurisdiction':'VN'})
    assert r.status_code==201,r.text
    assert r.json()['tenant_id']=='tenant-demo'
    assert len(get(client,'/v1/companies')['items'])==4
    assert client.post('/v1/companies',headers=AUTH,json={
        'code':'NEWCO','name':'Duplicate Synthetic Company'}).status_code==409
    assert client.post('/v1/companies',json={
        'code':'UNAUTH','name':'Unauthorized Company'}).status_code==401


def test_ui_root(client):
    r=client.get('/')
    assert r.status_code==200
    assert 'OrgAxis' in r.text

def test_single_company_tenant_seed(tmp_path,monkeypatch):
    monkeypatch.setenv('ORGAXIS_SEED_PROFILE','single_company')
    monkeypatch.setenv('ORGAXIS_DEMO_COMPANY_CODE','CFCPILOT')
    monkeypatch.setenv('ORGAXIS_DEMO_COMPANY_NAME','Cement Factory Pilot (SYNTHETIC)')
    with engine.connect(tmp_path/'one_company.sqlite3') as db:
        seed(db)
        all_companies=engine.companies(db)['items']
        assert [(e['code'],e['name']) for e in all_companies] == [
            ('CFCPILOT','Cement Factory Pilot (SYNTHETIC)')]
        assert db.execute('SELECT COUNT(*) FROM person').fetchone()[0]==1000
        assert db.execute("SELECT COUNT(*) FROM change_set WHERE domain='org_structure'").fetchone()[0]==2
        assert db.execute('SELECT COUNT(*) FROM hr_event').fetchone()[0]==80
        assert engine.manager_for_person(db,'EMP-0030','2026-11-18')['manager']['person_id']=='EMP-0010'
