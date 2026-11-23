//go:build windows
package main

import (
 "errors"
 "fmt"
 "runtime"
 "syscall"
 "unsafe"
)

// Uses winsqlite3.dll bundled with supported Windows 10 installations.
// No third-party Go packages, Python interpreter, Docker or external sqlite3.exe.
var sqliteLib = syscall.NewLazyDLL("winsqlite3.dll")
var (
 sqliteOpen = sqliteLib.NewProc("sqlite3_open_v2")
 sqliteClose = sqliteLib.NewProc("sqlite3_close_v2")
 sqlitePrepare = sqliteLib.NewProc("sqlite3_prepare_v2")
 sqliteFinalize = sqliteLib.NewProc("sqlite3_finalize")
 sqliteStep = sqliteLib.NewProc("sqlite3_step")
 sqliteBindText = sqliteLib.NewProc("sqlite3_bind_text")
 sqliteBindInt64 = sqliteLib.NewProc("sqlite3_bind_int64")
 sqliteBindDouble = sqliteLib.NewProc("sqlite3_bind_double")
 sqliteBindNull = sqliteLib.NewProc("sqlite3_bind_null")
 sqliteColumnCount = sqliteLib.NewProc("sqlite3_column_count")
 sqliteColumnName = sqliteLib.NewProc("sqlite3_column_name")
 sqliteColumnType = sqliteLib.NewProc("sqlite3_column_type")
 sqliteColumnText = sqliteLib.NewProc("sqlite3_column_text")
 sqliteColumnBytes = sqliteLib.NewProc("sqlite3_column_bytes")
 sqliteColumnInt64 = sqliteLib.NewProc("sqlite3_column_int64")
 sqliteColumnDouble = sqliteLib.NewProc("sqlite3_column_double")
 sqliteErrmsg = sqliteLib.NewProc("sqlite3_errmsg")
)
const (sqlOK=0;sqlROW=100;sqlDONE=101;sqlInteger=1;sqlFloat=2;sqlText=3;sqlBlob=4;sqlNull=5)
type nativeSQLite struct{handle uintptr}
func cString(p uintptr) string {
 if p==0 {return ""}; b:=make([]byte,0,64)
 for i:=uintptr(0); i<1048576; i++ {v:=*(*byte)(unsafe.Pointer(p+i));if v==0 {break};b=append(b,v)}
 return string(b)
}
func (d *nativeSQLite) errmsg() string {p,_,_:=sqliteErrmsg.Call(d.handle);return cString(p)}
func openSQLite(path string) (SQLiteDB,error) {
 p,e:=syscall.BytePtrFromString(path);if e!=nil{return nil,e}
 var handle uintptr
 rc,_,_:=sqliteOpen.Call(uintptr(unsafe.Pointer(p)),uintptr(unsafe.Pointer(&handle)),2|4|0x10000,0)
 runtime.KeepAlive(p)
 if rc!=sqlOK {if handle!=0 {sqliteClose.Call(handle)};return nil,fmt.Errorf("sqlite3_open_v2: %d",rc)}
 d:=&nativeSQLite{handle:handle}
 for _,p:=range []string{"PRAGMA foreign_keys=ON","PRAGMA busy_timeout=15000","PRAGMA journal_mode=WAL"} {if e:=d.Exec(p);e!=nil{d.Close();return nil,e}}
 return d,nil
}
func (d *nativeSQLite) Close() error {if d.handle==0 {return nil};rc,_,_:=sqliteClose.Call(d.handle);d.handle=0;if rc!=sqlOK{return fmt.Errorf("sqlite close status %d",rc)};return nil}
func (d *nativeSQLite) prepare(query string, args []any) (uintptr,error){
 q,e:=syscall.BytePtrFromString(query);if e!=nil{return 0,e}
 var stmt uintptr
 rc,_,_:=sqlitePrepare.Call(d.handle,uintptr(unsafe.Pointer(q)),^uintptr(0),uintptr(unsafe.Pointer(&stmt)),0)
 runtime.KeepAlive(q)
 if rc!=sqlOK{return 0,fmt.Errorf("sqlite prepare: %s",d.errmsg())}
 if stmt==0 {return 0,errors.New("empty sql statement")}
 for i,a:=range args {
  idx:=uintptr(i+1);var status uintptr
  switch v:=a.(type) {
  case nil:status,_,_=sqliteBindNull.Call(stmt,idx)
  case int: status,_,_=sqliteBindInt64.Call(stmt,idx,uintptr(int64(v)))
  case int64:status,_,_=sqliteBindInt64.Call(stmt,idx,uintptr(v))
  case float64:status,_,_=sqliteBindDouble.Call(stmt,idx,uintptr(*(*uint64)(unsafe.Pointer(&v))))
  case bool: n:=uintptr(0);if v{n=1};status,_,_=sqliteBindInt64.Call(stmt,idx,n)
  default:
   s:=[]byte(fmt.Sprint(a))
   status,_,_=sqliteBindText.Call(stmt,idx,uintptr(unsafe.Pointer(unsafe.SliceData(s))),uintptr(len(s)),^uintptr(0))
   runtime.KeepAlive(s)
  }
  if status!=sqlOK {sqliteFinalize.Call(stmt);return 0,fmt.Errorf("sqlite bind %d: %s",i,d.errmsg())}
 }
 return stmt,nil
}
func (d *nativeSQLite) Query(query string,args ...any)([]Row,error) {
 stmt,e:=d.prepare(query,args);if e!=nil{return nil,e};defer sqliteFinalize.Call(stmt)
 n,_,_:=sqliteColumnCount.Call(stmt);names:=make([]string,int(n));for i:=range names{p,_,_:=sqliteColumnName.Call(stmt,uintptr(i));names[i]=cString(p)}
 out:=make([]Row,0)
 for {
  rc,_,_:=sqliteStep.Call(stmt)
  if rc==sqlDONE{return out,nil}
  if rc!=sqlROW{return nil,fmt.Errorf("sqlite query: %s",d.errmsg())}
  row:=make(Row,len(names))
  for i,name:=range names {
   j:=uintptr(i);typ,_,_:=sqliteColumnType.Call(stmt,j)
   switch typ {
   case sqlNull:row[name]=nil
   case sqlInteger:v,_,_:=sqliteColumnInt64.Call(stmt,j);row[name]=int64(v)
   case sqlFloat: v,_,_:=sqliteColumnDouble.Call(stmt,j);bits:=uint64(v);row[name]=*(*float64)(unsafe.Pointer(&bits))
   case sqlText,sqlBlob:
    p,_,_:=sqliteColumnText.Call(stmt,j);ln,_,_:=sqliteColumnBytes.Call(stmt,j)
    if ln==0{row[name]=""}else if p!=0{row[name]=string(unsafe.Slice((*byte)(unsafe.Pointer(p)),int(ln)))}else{row[name]=""}
   }
  }
  out=append(out,row)
 }
}
func (d *nativeSQLite) Exec(query string,args ...any) error {
 stmt,e:=d.prepare(query,args);if e!=nil{return e};defer sqliteFinalize.Call(stmt)
 for {rc,_,_:=sqliteStep.Call(stmt);if rc==sqlDONE{return nil};if rc!=sqlROW{return fmt.Errorf("sqlite exec: %s",d.errmsg())}}
}
