"""Rebuild the forward-only demo backfill; run after enrich_seed.py and enrich_extended.py."""
from pathlib import Path
import gzip,sqlite3
root=Path(__file__).parent
c=sqlite3.connect(root/'assets/seed.sqlite3')
tables=['skill','person_skill','training_program','training_record','qualification','work_task','task_requirement','kpi_result','absence_case','recognition_case','discipline_case','occupation_period','workflow_request','pay_component','payroll_run','payroll_item','compensation_agreement','tax_profile','social_insurance_period','benefit_claim','external_work_history','immigration_credential','travel_document','work_authorization','entry_exit_event']
with gzip.open(root/'assets/demo_additions.sql.gz','wt',encoding='utf-8',compresslevel=9) as f:
    for pid,name,nation in c.execute('SELECT id,display_name,nationality FROM person WHERE synthetic=1 ORDER BY id'):
        qname=c.execute('SELECT quote(?)',(name,)).fetchone()[0]
        qid=c.execute('SELECT quote(?)',(pid,)).fetchone()[0]
        f.write(f"UPDATE person SET display_name={qname} WHERE id={qid} AND display_name LIKE 'Synthetic Employee %';\n")
        if nation!='VN':f.write(f"UPDATE person SET nationality='{nation}' WHERE id={qid} AND nationality='VN' AND synthetic=1;\n")
    for table in tables:
        for row in c.execute('SELECT * FROM '+table).fetchall():
            values=','.join(c.execute('SELECT quote(?)',(v,)).fetchone()[0] for v in row)
            f.write(f'INSERT OR IGNORE INTO {table} VALUES ({values});\n')
print('Produced',root/'assets/demo_additions.sql.gz')
