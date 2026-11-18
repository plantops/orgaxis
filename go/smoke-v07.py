#!/usr/bin/env python3
"""Run against an isolated synthetic demo. No real HR or device credentials."""
import json,sys,urllib.request,urllib.error,http.cookiejar
from pathlib import Path
base=sys.argv[1] if len(sys.argv)>1 else 'http://127.0.0.1:8769'
cred=Path(sys.argv[2]) if len(sys.argv)>2 else Path('data/DEMO-LOGIN-PRIVATE.txt')
users={}
for line in cred.read_text().splitlines():
 f=line.split()
 if len(f)==4 and f[0] in {'employee','hr','admin','payroll'}: users[f[0]]=f[-1]
assert len(users)==4,users.keys()
def login(name):
 jar=http.cookiejar.CookieJar(); op=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
 data=json.dumps({'username':name,'password':users[name]}).encode()
 res=op.open(urllib.request.Request(base+'/auth/login',data=data,headers={'Content-Type':'application/json'}))
 token=json.load(res)['csrf']
 return op,token
def call(op,method,path,payload=None,csrf=None):
 headers={'Content-Type':'application/json'}
 if csrf:headers['X-CSRF-Token']=csrf
 data=None if payload is None else json.dumps(payload).encode()
 req=urllib.request.Request(base+path,data=data,method=method,headers=headers)
 try:
  with op.open(req,timeout=10) as res:return res.status,json.load(res) if res.headers.get('Content-Type','').startswith('application/json') else res.read()
 except urllib.error.HTTPError as e:return e.code,json.load(e)
emp,et=login('employee');hr,ht=login('hr');admin,at=login('admin')
checks=[]
def check(name,condition):
 checks.append([name,'PASS' if condition else 'FAIL'])
 assert condition,name
s,j=call(emp,'GET','/v2/time/day?date=2026-11-02');check('employee timecard',s==200 and j['person_id']=='EMP-0012')
s,j=call(emp,'GET','/v2/time/day?person_id=EMP-0001&date=2026-11-02');check('employee cannot read another timecard',s==403)
s,j=call(emp,'GET','/v2/time/quality');check('employee cannot see device logs',s==403)
s,j=call(hr,'GET','/v2/time/quality');check('HR device quality',s==200 and j['raw_events']==11977 and j['roster_rows']==6000)
s,j=call(emp,'GET','/v2/time/pay-preview?period=2026-11');check('pay period preview blocked',s==200 and j['blocked'] is True)
s,j=call(emp,'GET','/v2/time/pay-preview?period=2026-11&person_id=EMP-0001');check('cannot see another payroll',s==403)
b={'source':'SMOKE','device_id':'SIM-GATE-01','events':[{'event_id':'smoke-888','badge_id':'EMP-0012','direction':'IN','occurred_at':'2026-11-08T08:00:00+07:00'}]}
s,j=call(hr,'POST','/v2/time/punches/import',b);check('import CSRF required',s==403)
s,j=call(hr,'POST','/v2/time/punches/import',b,ht);check('import accepted',s==201 and j['accepted']==1)
s,j=call(hr,'POST','/v2/time/punches/import',b,ht);check('replay deduplicated',s==201 and j['duplicate']==1)
b['events'][0]['direction']='OUT'
s,j=call(hr,'POST','/v2/time/punches/import',b,ht);check('conflicting event rejected',s==409)
print(json.dumps({'status':'PASS','tests':checks},ensure_ascii=False,indent=2))
