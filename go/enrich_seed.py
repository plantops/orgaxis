"""Deterministic synthetic OrgAxis module seed. NEVER feed real HR information."""
from __future__ import annotations
import sqlite3, pathlib, random, shutil, json, datetime
R=pathlib.Path(__file__).parent
DB=R/'assets/seed.sqlite3'
T='tenant-cfc-pilot'
random.seed(9027)
N=1000

new_schema='''
CREATE TABLE IF NOT EXISTS workflow_request (
 id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenant(id),
 subject_person_id TEXT NOT NULL REFERENCES person(id),
 requester_id TEXT NOT NULL REFERENCES person(id),
 approver_id TEXT REFERENCES person(id),
 request_type TEXT NOT NULL, summary TEXT NOT NULL,
 requested_on TEXT NOT NULL, effective_date TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('pending','approved','rejected','unresolved')),
 decided_at TEXT, decision_note TEXT,
 routing_policy TEXT NOT NULL DEFAULT 'LINE_MANAGER',
 CHECK (length(summary) BETWEEN 3 AND 300)
);
CREATE INDEX IF NOT EXISTS idx_workflow_status ON workflow_request(tenant_id,status,requested_on);
CREATE INDEX IF NOT EXISTS idx_training_person_date ON training_record(tenant_id,person_id,completed_on);
CREATE INDEX IF NOT EXISTS idx_kpi_person_period ON kpi_result(tenant_id,person_id,period);
CREATE INDEX IF NOT EXISTS idx_skill_person ON person_skill(tenant_id,person_id);
CREATE INDEX IF NOT EXISTS idx_absence_period ON absence_case(tenant_id,valid_from,approval_status);
'''

def enrich(c):
    c.executescript(new_schema)
    if c.execute('select count(*) from skill').fetchone()[0] > 0:
        print('Seed already present; skipping')
        return
    # Realistic-sounding names generated from parts; all records remain explicitly synthetic.
    family=['Nguyễn','Trần','Lê','Phạm','Hoàng','Vũ','Đỗ','Bùi','Đặng','Phan','Ngô','Hồ']
    middle=['Văn','Thị','Minh','Đức','Quang','Ngọc','Xuân','Hữu','Thanh','Đình']
    given=['An','Bình','Chi','Dũng','Giang','Hải','Hùng','Khánh','Lâm','Lan','Long','Mai','Nam','Phong','Quân','Sơn','Thành','Thảo','Thủy','Trang','Tú','Vinh']
    for i in range(1,N+1):
        name=f'{family[(i*7+3)%len(family)]} {middle[(i*13+2)%len(middle)]} {given[(i*17+i//23)%len(given)]} (Demo {i:04d})'
        c.execute('UPDATE person SET display_name=? WHERE id=? AND display_name LIKE ?', (name,f'EMP-{i:04d}','Synthetic Employee %'))
    skills=[
        ('STOCK','Quản lý kho & FIFO'),('FORK','Lái xe nâng'),('TRUCK','Lái xe tải'),('PACK','Vận hành đóng bao'),
        ('CRANE','Vận hành cẩu & nâng hạ'),('MECH','Bảo trì cơ khí'),('ELEC','An toàn và bảo trì điện'),
        ('KILN','Vận hành lò nung'),('QC','Lấy mẫu & kiểm tra chất lượng'),('HSE','Đánh giá rủi ro HSE'),
        ('FIRE','Ứng phó PCCC'),('CHEM','Xử lý hóa chất'),('SPILL','Ứng phó sự cố tràn dầu'),
        ('LOTO','Cô lập năng lượng LOTO'),('LEAD','Điều hành ca'),('DIGITAL','ERP / dữ liệu vận hành')]
    c.executemany('INSERT INTO skill (id,tenant_id,code,label) VALUES (?,?,?,?)',[(f'SK-{a}',T,a,b) for a,b in skills])
    programs=[
        ('ATVSLD','An toàn vệ sinh lao động (nhóm phù hợp)','NĐ 44/2016; xác định nhóm theo nhiệm vụ'),
        ('PCCC','Phòng cháy chữa cháy & CNCH','Theo đối tượng và văn bản hiệu lực'),
        ('HOA_CHAT','An toàn hóa chất','Theo đối tượng và văn bản hiệu lực'),
        ('TRAN_DAU','Ứng phó sự cố tràn dầu','Kế hoạch ứng phó của đơn vị'),
        ('XE_NANG','Vận hành thiết bị nâng','Theo thiết bị và điều kiện áp dụng'),
        ('DIEN','An toàn điện','Theo công việc và điều kiện áp dụng'),
        ('LOTO','Cô lập năng lượng / LOTO','Quy trình nội bộ'),
        ('LAM_CAO','Làm việc trên cao','Quy trình nội bộ'),
        ('CONFINED','Không gian hạn chế','Quy trình nội bộ'),
        ('PACKER','An toàn vận hành máy đóng bao','SOP đóng bao'),
        ('ISO','ISO 9001 / 14001 / 45001','Chương trình nội bộ'),
        ('ERP','ERP & dữ liệu vận hành','Chương trình nội bộ')]
    c.executemany('INSERT INTO training_program(id,tenant_id,name,regulatory_basis,version) VALUES (?,?,?,?,?)',[(f'TP-{code}',T,title,basis,'demo-2026') for code,title,basis in programs])
    task_defs=[('STOCK','Thủ kho'),('FORK','Vận hành xe nâng'),('TRUCK','Lái xe tải'),('PACK','Vận hành đóng bao'),('MTN','Bảo trì thiết bị'),('QC','Lấy mẫu QC'),('HSE','Kiểm tra HSE')]
    c.executemany('INSERT INTO work_task(id,tenant_id,code,name) VALUES (?,?,?,?)',[(f'TASK-{code}',T,code,name) for code,name in task_defs])
    c.executemany('INSERT INTO task_requirement(task_id,skill_id,min_level) VALUES (?,?,?)',[(f'TASK-{code}',f'SK-{skill}',level) for code,skill,level in [('STOCK','STOCK',3),('FORK','FORK',3),('TRUCK','TRUCK',3),('PACK','PACK',2),('MTN','MECH',3),('QC','QC',2),('HSE','HSE',3)]])

    occupied=c.execute('''SELECT a.person_id,p.code,u.code FROM assignment a JOIN position p ON p.id=a.position_id
      JOIN position_placement pl ON pl.position_id=a.position_id AND pl.valid_to IS NULL
      JOIN org_unit u ON u.id=pl.org_unit_id
      WHERE a.valid_to IS NULL AND a.assignment_type='primary' ORDER BY a.person_id''').fetchall()
    people = {p:(pos,unit) for p,pos,unit in occupied}
    skill_map={'WH':['STOCK','FORK','TRUCK','FIRE','SPILL'],'PACK':['PACK','FORK','FIRE','LOTO'],'MTN':['MECH','ELEC','LOTO','FIRE'],'HSE':['HSE','FIRE','CHEM','SPILL'],'LOG':['TRUCK','FORK','STOCK','FIRE'],'OPS':['KILN','QC','LOTO','FIRE'],'P1':['KILN','LOTO','HSE'],'P2':['KILN','LOTO','HSE']}
    metrics=['SAFETY','QUALITY','PRODUCTIVITY','5S','ATTENDANCE']
    for i in range(1,N+1):
        pid=f'EMP-{i:04d}'; pos,unit=people.get(pid,('OPS-OP','OPS'))
        options=skill_map.get(unit,['LEAD','DIGITAL','HSE','FIRE'])[:]
        # Seed cross-training independent of the current position.
        if i%8==0: options+=['PACK','FORK']
        if i%11==0: options+=['TRUCK','STOCK']
        if 'MGR' in pos or 'LEAD' in pos or 'CEO' in pos: options+=['LEAD','DIGITAL']
        for j,skill in enumerate(dict.fromkeys(options)):
            if (i+j)%5==0: continue
            level=1+((i+j*3)%4)
            assessed=f'2026-{1+(i+j)%9:02d}-{1+(i*3+j)%26:02d}'
            expires='2027-12-31' if j%3==0 else None
            c.execute('INSERT INTO person_skill VALUES (?,?,?,?,?,?,?,?)',(f'PS-{i:04d}-{skill}',T,pid,f'SK-{skill}',level,assessed,expires,None))
        # Historical training is not blanket certification: scores/results are individual.
        tcodes=['ATVSLD','PCCC','ISO','ERP']
        tcodes += (['XE_NANG','TRAN_DAU'] if unit in ('WH','LOG') else ['PACKER'] if unit=='PACK' else ['DIEN','LOTO'] if unit=='MTN' else ['HOA_CHAT','PCCC'] if unit=='HSE' else ['LOTO'])
        for j,code in enumerate(dict.fromkeys(tcodes)):
            if (i+j)%7==0: continue
            comp=f'2026-{1+((i+j*2)%10):02d}-{1+(i+j*3)%25:02d}'
            expiry= ('2026-11-25' if i%19==0 else '2027-10-31' if i%3==0 else '2028-06-30')
            outcome='failed' if (i*3+j)%39==0 else 'passed' if (i+j)%6 else 'attended'
            c.execute('INSERT INTO training_record VALUES (?,?,?,?,?,?,?,?)',(f'TR-{i:04d}-{code}',T,pid,f'TP-{code}',comp,outcome,expiry,None))
        if i%5!=0:
            qtype='XE_NANG' if unit in ('WH','LOG') else 'DIEN' if unit=='MTN' else 'PCCC'
            c.execute('INSERT INTO qualification VALUES (?,?,?,?,?,?,?,?)',(f'QUAL-{i:04d}',T,pid,qtype,'2026-01-15', '2026-11-30' if i%13==0 else '2028-01-15','valid',None))
        for period in ['2026-09','2026-10']:
            for j,metric in enumerate(metrics):
                if metric=='PRODUCTIVITY' and ('MGR' in pos or pos=='CEO'): continue
                target=100.0
                actual=round(78+(i*11+j*17+int(period[-2:])*3)%45,1)
                c.execute('INSERT INTO kpi_result VALUES (?,?,?,?,?,?,?,?)',(f'KPI-{i:04d}-{period}-{metric}',T,pid,period,metric,actual,target,'synthetic_seed'))
        if i<=175:
            d=f'2026-{5+(i%6):02d}-{1+(i%17):02d}'
            end=(datetime.date.fromisoformat(d)+datetime.timedelta(days=1+i%5)).isoformat()
            kind=['annual','sick','maternity','unpaid'][i%4]
            status=['approved','pending','rejected'][i%3]
            c.execute('INSERT INTO absence_case VALUES (?,?,?,?,?,?,?,?)',(f'ABS-{i:04d}',T,f'EE-{i:04d}',kind,d,end,status,f'SYNTHETIC-ABS-{i:04d}'))
        if i%11==0:
            c.execute('INSERT INTO recognition_case VALUES (?,?,?,?,?,?,?)',(f'REW-{i:04d}',T,pid,'Kaizen' if i%2 else 'Safety excellence','2026-10-15','approved' if i%3 else 'proposed',None))
        if i%83==0:
            c.execute('INSERT INTO discipline_case VALUES (?,?,?,?,?,?,?,?)',(f'DISC-{i:04d}',T,pid,'2026-09-18','under_review',None,None,None))
        # This is classification demo data, not occupational/legal certification.
        grade=['I','II','III','IV','V','VI','UNKNOWN'][(i+2)%7]
        occupation=pos.replace('-',' ')
        c.execute('INSERT INTO occupation_period VALUES (?,?,?,?,?,?,?,?,?,?)',(f'OCC-{i:04d}',T,pid,'Cement Factory Pilot (SYNTHETIC)',occupation,grade,'2026-01-01',None,None,'DEMO-NOT-VERIFIED'))
        if i%17==0:
            c.execute('INSERT INTO occupation_period VALUES (?,?,?,?,?,?,?,?,?,?)',(f'OCC-EXT-{i:04d}',T,pid,'Previous employer (SELF REPORTED)','Industrial worker','UNKNOWN','2018-01-01','2022-12-31',None,'UNVERIFIED-SYNTHETIC'))
    # Process requests with authentic looking pending states but all synthetic.
    for i in range(15,110,3):
        pid=f'EMP-{i:04d}'; approver=f'EMP-{(i%10)+1:04d}'
        c.execute('INSERT INTO workflow_request(id,tenant_id,subject_person_id,requester_id,approver_id,request_type,summary,requested_on,effective_date,status) VALUES (?,?,?,?,?,?,?,?,?,?)',
          (f'WF-{i:04d}',T,pid,pid,approver,'leave' if i%2 else 'shift_assignment',f'Đề nghị của nhân sự {pid} (dữ liệu giả lập)','2026-11-10','2026-11-20', 'pending' if i%5 else 'approved'))
    c.commit()

if __name__=='__main__':
    conn=sqlite3.connect(DB)
    conn.execute('PRAGMA foreign_keys=ON')
    enrich(conn)
    for tbl in ['person','person_skill','training_record','kpi_result','absence_case','recognition_case','discipline_case','occupation_period','workflow_request']:
        print(tbl,conn.execute('SELECT COUNT(*) FROM '+tbl).fetchone()[0])
    conn.close()
