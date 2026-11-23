"""Reproducible 1,000-person completely synthetic fixture. No real PII."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

from engine import connect, setup, TENANT, utc_now

BASE = '2026-01-01'

def seed(con):
    setup(con)
    if con.execute('SELECT 1 FROM tenant WHERE id=?', (TENANT,)).fetchone():
        raise RuntimeError('Database already seeded. Use --reset on a DEMO database only.')
    con.execute('BEGIN')
    try:
        con.execute('INSERT INTO tenant VALUES (?,?,?)',
            (TENANT,os.getenv('ORGAXIS_TENANT_CODE','DEMO'),
             os.getenv('ORGAXIS_TENANT_NAME','Fictional Cement Group - synthetic data')))
        for name,mode in [
            ('organization','core'),('employment','core'),('workflow','core'),
            ('competency','off'),('training','off'),('performance','off'),
            ('reward','off'),('discipline','off'),('occupational_history','off'),
            ('compensation','off'),('payroll','off'),('benefits','off'),
            ('tax','off'),('immigration','off'),('shift_dispatch','off')]:
            con.execute('INSERT INTO feature_setting VALUES (?,?,?)', (TENANT,name,mode))
        entities = [
            ('LE-CFC','CFC','Demo Cement Manufacturing Co.'),
            ('LE-SVC','SVC','Demo Industrial Services Co.'),
            ('LE-HLD','HLD','Demo Holdings Co.')]
        if os.getenv('ORGAXIS_SEED_PROFILE','group') == 'single_company':
            entities = [('LE-CFC', os.getenv('ORGAXIS_DEMO_COMPANY_CODE','CFCPILOT'),
                         os.getenv('ORGAXIS_DEMO_COMPANY_NAME','Cement Factory Pilot (SYNTHETIC)'))]
        con.executemany('INSERT INTO legal_entity(id,tenant_id,code,name) VALUES (?,?,?,?)',
                        [(id,TENANT,code,name) for id,code,name in entities])
        for i,child in enumerate(('LE-CFC','LE-SVC') if len(entities)>1 else ()):
            con.execute('''INSERT INTO corporate_relationship VALUES (?,?,?,?,?,?,?)''',
                        (f'CORP-{i}',TENANT,'LE-HLD',child,'CONTROL',BASE,None))
        units = [
            ('CORP','Corporate Office','GROUP',BASE),('OPS','Operations','DIVISION',BASE),
            ('P1','Plant 1','PLANT',BASE),('P2','Plant 2','PLANT',BASE),
            ('WH','Warehouse','DEPARTMENT',BASE),('PACK','Packing','DEPARTMENT',BASE),
            ('MTN','Maintenance','DEPARTMENT',BASE),('HSE','HSE','DEPARTMENT',BASE),
            ('LOG','Logistics','DEPARTMENT','2027-01-01')]
        con.executemany('''INSERT INTO org_unit(id,tenant_id,legal_entity_id,code,name,unit_type,created_at)
          VALUES (?,?,?,?,?,?,?)''',
          [(f'OU-{code}',TENANT,'LE-CFC',code,label,kind,created) for code,label,kind,created in units])
        initial_edges = [('CORP','OPS'),('CORP','HSE'),('OPS','P1'),('OPS','P2'),
                         ('P1','WH'),('P1','PACK'),('P1','MTN')]
        for i,(parent,child) in enumerate(initial_edges):
            end = '2026-07-01' if child == 'WH' else None
            con.execute('''INSERT INTO org_unit_relation(id,tenant_id,parent_id,child_id,valid_from,valid_to)
              VALUES (?,?,?,?,?,?)''', (f'OUR-{i}',TENANT,f'OU-{parent}',f'OU-{child}',BASE,end))
        jobs = ['Executive','Management','Supervisor','Operations','Technical']
        for job in jobs:
            con.execute('INSERT INTO job_profile VALUES (?,?,?,?)',
                        (f'JOB-{job.upper()}',TENANT,job,job))
        # code, title, unit, parent_position, holder_capacity, job_family, start_date
        slots = [
            ('CEO','Chief Executive','CORP',None,1,'Executive',BASE),
            ('OPS-MGR','Operations Director','OPS','CEO',1,'Management',BASE),
            ('P1-MGR','Plant 1 Director','P1','OPS-MGR',1,'Management',BASE),
            ('P2-MGR','Plant 2 Director','P2','OPS-MGR',1,'Management',BASE),
            ('WH-MGR','Warehouse Manager','WH','P1-MGR',1,'Management',BASE),
            ('PACK-MGR','Packing Manager','PACK','P1-MGR',1,'Management',BASE),
            ('MTN-MGR','Maintenance Manager','MTN','P1-MGR',1,'Management',BASE),
            ('HSE-MGR','HSE Manager','HSE','CEO',1,'Management',BASE),
            ('WH-LEAD','Warehouse Shift Leader','WH','WH-MGR',1,'Supervisor',BASE),
            ('PACK-LEAD','Packing Shift Leader','PACK','PACK-MGR',1,'Supervisor',BASE),
            ('MTN-LEAD','Maintenance Shift Leader','MTN','MTN-MGR',1,'Supervisor',BASE),
            ('WH-OP','Warehouse Operator','WH','WH-LEAD',500,'Operations',BASE),
            ('PACK-OP','Packing Operator','PACK','PACK-LEAD',600,'Operations',BASE),
            ('MTN-OP','Maintenance Technician','MTN','MTN-LEAD',200,'Technical',BASE),
            ('OPS-OP','Operations Worker','OPS','OPS-MGR',200,'Operations',BASE),
            ('LOG-MGR','Logistics Manager','LOG','OPS-MGR',1,'Management','2027-01-01'),
            ('LOG-OP','Logistics Operator','LOG','LOG-MGR',100,'Operations','2027-01-01')]
        for code,title,unit,parent,capacity,job,start in slots:
            con.execute('''INSERT INTO position(id,tenant_id,job_profile_id,code,title,holder_limit,created_at)
              VALUES (?,?,?,?,?,?,?)''', (f'POS-{code}',TENANT,f'JOB-{job.upper()}',code,title,capacity,start))
            con.execute('''INSERT INTO position_placement(id,tenant_id,position_id,org_unit_id,valid_from)
              VALUES (?,?,?,?,?)''', (f'PP-{code}',TENANT,f'POS-{code}',f'OU-{unit}',start))
            con.execute('''INSERT INTO staffing_authorization(id,tenant_id,position_id,approved_fte,valid_from)
              VALUES (?,?,?,?,?)''', (f'SA-{code}',TENANT,f'POS-{code}',capacity,start))
            if parent:
                end = '2026-07-01' if code == 'WH-MGR' else None
                con.execute('''INSERT INTO position_reporting(id,tenant_id,position_id,supervisor_position_id,
                  valid_from,valid_to) VALUES (?,?,?,?,?,?)''',
                  (f'PR-{code}',TENANT,f'POS-{code}',f'POS-{parent}',start,end))
        # Exactly two *blank-org* structural change sets; first is effective in 2026,
        # second is an approved 2027 future structure with two vacant positions.
        org_cases=[('ORG-001','Move Warehouse under Operations','2026-07-01'),
                   ('ORG-002','Open Logistics Department','2027-01-01')]
        for cs,title,eff in org_cases:
            con.execute('''INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,
                scenario,recorded_at,source_ref) VALUES (?,?,?,?,?,'published','official',?,?)''',
                (cs,TENANT,'org_structure',title,eff,utc_now(),f'SYNTHETIC-{cs}'))
        con.execute('''INSERT INTO org_unit_relation(id,tenant_id,parent_id,child_id,
           valid_from,change_set_id) VALUES (?,?,?,?,?,?)''',
           ('OUR-WH-NEW',TENANT,'OU-OPS','OU-WH','2026-07-01','ORG-001'))
        con.execute('''INSERT INTO position_reporting(id,tenant_id,position_id,supervisor_position_id,
           valid_from,change_set_id) VALUES (?,?,?,?,?,?)''',
           ('PR-WH-MGR-NEW',TENANT,'POS-WH-MGR','POS-OPS-MGR','2026-07-01','ORG-001'))
        con.execute('''INSERT INTO org_unit_relation(id,tenant_id,parent_id,child_id,
           valid_from,change_set_id) VALUES (?,?,?,?,?,?)''',
           ('OUR-LOG',TENANT,'OU-OPS','OU-LOG','2027-01-01','ORG-002'))
        con.execute('''UPDATE position_placement SET change_set_id='ORG-002' WHERE position_id IN
           ('POS-LOG-MGR','POS-LOG-OP')''')
        con.execute('''UPDATE position_reporting SET change_set_id='ORG-002' WHERE position_id IN
           ('POS-LOG-MGR','POS-LOG-OP')''')
        for i,changes in [('ORG-001', [('org_unit_relation','OU-WH','reparent',{'from':'OU-P1','to':'OU-OPS'}),
                                         ('position_reporting','POS-WH-MGR','reparent',{'from':'POS-P1-MGR','to':'POS-OPS-MGR'})]),
                          ('ORG-002', [('org_unit','OU-LOG','create',{}),('position','POS-LOG-MGR','create',{}),
                                         ('position','POS-LOG-OP','create',{})])]:
            for j,(entity,key,operation,details) in enumerate(changes):
                con.execute('INSERT INTO change_item VALUES (?,?,?,?,?,?)',
                            (f'{i}-ITEM-{j}',i,entity,key,operation,json.dumps(details)))
        manager_positions = ['CEO','OPS-MGR','P1-MGR','P2-MGR','WH-MGR','PACK-MGR',
                             'MTN-MGR','HSE-MGR','WH-LEAD','PACK-LEAD','MTN-LEAD']
        for n in range(1,1001):
            pid = f'EMP-{n:04d}'
            pos = (manager_positions[n-1] if n <= len(manager_positions) else
                   'WH-OP' if n <= 400 else 'PACK-OP' if n <= 800 else
                   'MTN-OP' if n <= 900 else 'OPS-OP')
            con.execute('INSERT INTO person(id,tenant_id,display_name) VALUES (?,?,?)',
                        (pid,TENANT,f'Synthetic Employee {n:04d}'))
            con.execute('''INSERT INTO employment_episode(id,tenant_id,person_id,legal_entity_id,valid_from)
               VALUES (?,?,?,?,?)''', (f'EE-{n:04d}',TENANT,pid,'LE-CFC',BASE))
            end = None
            if 30 <= n <= 109:
                i = n - 30
                end = f'2026-{3+i//10:02d}-{5+i%10:02d}'
            con.execute('''INSERT INTO assignment(id,tenant_id,person_id,employment_id,
              position_id,assignment_type,fte,valid_from,valid_to)
              VALUES (?,?,?,?,?,'primary',1,?,?)''',
              (f'AS-{n:04d}-1',TENANT,pid,f'EE-{n:04d}',f'POS-{pos}',BASE,end))
        # Exactly 80 historical workforce changes (80 distinct people).
        for n in range(30,110):
            i=n-30
            pid=f'EMP-{n:04d}'
            eff=f'2026-{3+i//10:02d}-{5+i%10:02d}'
            cs=f'PERSON-CS-{n:04d}'
            con.execute('''INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,
              recorded_at,source_ref) VALUES (?,?,?,?,?,'published',?,?)''',
              (cs,TENANT,'workforce',f'Transfer {pid}',eff,utc_now(),f'SYNTHETIC-{cs}'))
            con.execute('''INSERT INTO assignment(id,tenant_id,person_id,employment_id,
              position_id,assignment_type,fte,valid_from,change_set_id)
              VALUES (?,?,?,?,?,'primary',1,?,?)''',
              (f'AS-{n:04d}-2',TENANT,pid,f'EE-{n:04d}','POS-PACK-OP',eff,cs))
            con.execute('''INSERT INTO hr_event(id,tenant_id,person_id,event_type,
               effective_date,status,change_set_id,details)
               VALUES (?,?,?,?,?,'published',?,?)''',
               (f'HE-{n:04d}',TENANT,pid,'transfer',eff,cs,
                json.dumps({'from':'WH-OP','to':'PACK-OP','synthetic':True})))
        con.execute('COMMIT')
    except Exception:
        con.execute('ROLLBACK')
        raise


def stats(con):
    return {name:con.execute(f'SELECT COUNT(*) FROM {name}').fetchone()[0]
            for name in ('tenant','legal_entity','org_unit','position','person',
                         'employment_episode','assignment','hr_event','change_set')}


def main():
    parser=argparse.ArgumentParser(description='Generate a synthetic HR temporal database')
    parser.add_argument('--db',default=str(Path(__file__).resolve().parent/'demo.sqlite3'))
    parser.add_argument('--reset',action='store_true',help='DELETE selected DEMO database first')
    args=parser.parse_args()
    path=Path(args.db).resolve()
    if args.reset:
        for suffix in ('','-wal','-shm'):
            (Path(str(path)+suffix)).unlink(missing_ok=True)
    if path.exists() and path.stat().st_size:
        raise SystemExit('DB already exists; use --reset for synthetic demo data only')
    path.parent.mkdir(parents=True,exist_ok=True)
    con=connect(path)
    try:
        seed(con)
        print(json.dumps({'db':str(path),'counts':stats(con),'org_structure_changes':2,
                          'personnel_events':80,'synthetic':True},indent=2))
    finally:
        con.close()

if __name__=='__main__':
    main()
