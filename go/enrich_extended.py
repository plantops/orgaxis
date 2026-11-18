"""Extend the synthetic HR fixture with finance and global mobility cases. No statutory tax/payroll formula."""
import sqlite3
from pathlib import Path
p=Path(__file__).parent/'assets/seed.sqlite3'
c=sqlite3.connect(p)
T='tenant-cfc-pilot'
if c.execute('SELECT COUNT(*) FROM compensation_agreement').fetchone()[0]:
    raise SystemExit('Already enhanced')
components=[('BASE','Lương cơ bản','earning'),('ALLOW','Phụ cấp','earning'),('BONUS','Thưởng mô phỏng','earning')]
for code,name,kind in components:
    c.execute('INSERT INTO pay_component VALUES (?,?,?,?,?)',(f'PAY-{code}',T,code,name,kind))
c.execute('INSERT INTO payroll_run VALUES (?,?,?,?,?,?)',('RUN-2026-10',T,'LE-CFC','2026-10','sample_draft','DEMO-ONLY-NO-LEGAL-CALC'))
for i in range(1,1001):
    p=f'EMP-{i:04d}';ee=f'EE-{i:04d}'
    amount=6500000+((i*31)%48)*350000
    c.execute('INSERT INTO compensation_agreement VALUES (?,?,?,?,?,?,?,?)',(f'COMP-{i:04d}',T,ee,'VND',amount,'2026-01-01',None,'SYNTHETIC-NON-AUTHORITATIVE'))
    c.execute('INSERT INTO tax_profile VALUES (?,?,?,?,?,?,?,?)',(f'TAX-{i:04d}',T,p,'VN','resident' if i%13 else 'unknown',i%4,'2026-01-01',None))
    c.execute('INSERT INTO social_insurance_period VALUES (?,?,?,?,?,?,?,?)',(f'SI-{i:04d}',T,p,'LE-CFC','2026-01-01',None,'self_reported',None))
    # Payroll records are synthetic source values, not payslips and not statutory computations.
    c.execute('INSERT INTO payroll_item VALUES (?,?,?,?,?,?,?)',(f'PI-{i:04d}', 'RUN-2026-10',ee,'PAY-BASE',amount,'VND','SYNTHETIC-MOCK-NOT-PAYSLIP'))
    if i%20==0:
        c.execute('INSERT INTO benefit_claim VALUES (?,?,?,?,?,?,?,?,?,?)',(f'BEN-{i:04d}',T,p,f'ABS-{i:04d}' if i<=175 else None,'sickness' if i%3 else 'maternity','2026-10-20','pending',None,'DEMO-UNVERIFIED',None))
    if i%17==0:
        c.execute('INSERT INTO external_work_history VALUES (?,?,?,?,?,?,?,?)',(f'EWH-{i:04d}',T,p,'External industrial employer (SELF-REPORTED)','Factory operator','2018-01-01','2022-12-31',None))
for j,i in enumerate(range(120,138)):
    p=f'EMP-{i:04d}';country=['JP','CN','TW','KR','IN','US'][j%6]
    c.execute('UPDATE person SET nationality=? WHERE id=? AND synthetic=1',(country,p))
    c.execute('INSERT INTO travel_document VALUES (?,?,?,?,?,?,?,?)',(f'DOC-{i:04d}',T,p,'PASSPORT','XXXX'+str(9000+i),country,'2028-11-18','unverified_demo'))
    c.execute('INSERT INTO work_authorization VALUES (?,?,?,?,?,?,?,?,?,?,?,?)',(f'WA-{i:04d}',T,p,'LE-CFC','PERMIT' if i%3 else 'EXEMPTION_CONFIRMATION','sample_unverified','Engineering consultant','P1','2026-01-01','2027-03-31', 'DEMO-ONLY',None))
    c.execute('INSERT INTO immigration_credential VALUES (?,?,?,?,?,?,?,?,?)',(f'IMM-{i:04d}',T,p,'LE-CFC','RESIDENCE','MASKED-'+str(i),'2026-01-01','2027-11-18','demo_unverified'))
    c.execute('INSERT INTO entry_exit_event VALUES (?,?,?,?,?,?)',(f'ENTRY-{i:04d}',T,p,'ENTRY','2026-05-06T10:00:00','synthetic'))
c.commit()
for t in ['compensation_agreement','payroll_item','benefit_claim','tax_profile','social_insurance_period','immigration_credential','work_authorization','travel_document','external_work_history']:
 print(t,c.execute('select count(*) from '+t).fetchone()[0])
c.close()
