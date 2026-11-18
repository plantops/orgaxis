"""Upgrade the embedded, 100%-synthetic demo SQLite snapshot for fast first boot.
This creates no statutory pay record and never touches production data.
"""
import sqlite3
from pathlib import Path
p=Path(__file__).parent/'assets/seed.sqlite3'
c=sqlite3.connect(p)
assert c.execute('select count(*) from person where synthetic=1').fetchone()[0]>=1000
c.execute('''CREATE TABLE IF NOT EXISTS payroll_demo_ledger(tenant_id TEXT NOT NULL, person_id TEXT NOT NULL, period TEXT NOT NULL,
  base_pay INTEGER NOT NULL, allowance INTEGER NOT NULL, overtime INTEGER NOT NULL, attendance_bonus INTEGER NOT NULL,
  performance_bonus INTEGER NOT NULL,gross INTEGER NOT NULL,employee_insurance INTEGER NOT NULL,
  other_deduction INTEGER NOT NULL,taxable_income INTEGER NOT NULL,pit_estimate INTEGER NOT NULL,net_estimate INTEGER NOT NULL,
  dependents INTEGER NOT NULL,rule_version TEXT NOT NULL,PRIMARY KEY(tenant_id,person_id,period))''')
c.execute('CREATE INDEX IF NOT EXISTS idx_demo_pay_period ON payroll_demo_ledger(tenant_id,period)')
def pit(x):
    widths=[5_000_000,5_000_000,8_000_000,14_000_000,20_000_000,28_000_000]
    rates=[.05,.10,.15,.20,.25,.30,.35]
    res=0
    for width,rate in zip(widths+[x],rates):
        take=min(max(0,x),width);res+=take*rate;x-=take
        if x<=0:break
    return int(res+0.5)
for id,base,deps in c.execute('''SELECT p.id,c.base_pay_minor,t.dependent_count FROM person p
    JOIN employment_episode ee ON ee.person_id=p.id JOIN compensation_agreement c ON c.employment_id=ee.id
    JOIN tax_profile t ON t.person_id=p.id WHERE p.synthetic=1 ORDER BY p.id''').fetchall():
    i=int(id[4:]);
    for month in (9,10,11):
        allowance=450_000+(i%7)*130_000;overtime=((i*7+month)%9)*115_000
        attendance=0 if i%11==0 else 350_000
        bonus=((i*13+month)%12)*145_000+(1_500_000 if i%25==0 else 0)
        gross=base+allowance+overtime+attendance+bonus
        insurance=int(base*.105+.5);other=i%4*50_000
        taxable=max(0,gross-insurance-11_000_000-deps*4_400_000)
        tax=pit(taxable);net=gross-insurance-other-tax
        c.execute('''INSERT OR IGNORE INTO payroll_demo_ledger VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)''',
                  ('tenant-cfc-pilot',id,f'2026-{month:02d}',base,allowance,overtime,attendance,bonus,gross,insurance,other,taxable,tax,net,deps,'ILLUSTRATIVE-2026-NOT-STATUTORY'))
names=['Haruto Tanaka','Li Wei','Chen Yu-Hsuan','Kim Min-jun','Arjun Mehta','Daniel Miller','Yuki Nakamura','Zhang Yiming','Lin Chia-Hao','Park Seo-yeon','Priya Sharma','Emily Carter','Kenji Sato','Wang Xiaoyu','Huang Pei-Ling','Lee Ji-hoon','Rohan Patel','Michael Anderson']
for i,name in enumerate(names,120):
    c.execute("UPDATE person SET display_name=? WHERE id=? AND synthetic=1 AND nationality!='VN' AND display_name LIKE '%(Demo %'",(name+' (Demo)',f'EMP-{i:04d}'))
c.commit()
print('payroll ledger rows:',c.execute('select count(*) from payroll_demo_ledger').fetchone()[0])
print('international names:',c.execute("select group_concat(display_name, ', ') from person where nationality != 'VN'").fetchone()[0])
c.close()
