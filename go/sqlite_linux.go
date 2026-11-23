//go:build linux && cgo
package main
/*
#cgo LDFLAGS: -lsqlite3
#include <stdlib.h>
#include <sqlite3.h>
*/
import "C"
import (
 "fmt"
 "unsafe"
)
const (sqlROW=100;sqlDONE=101)
type nativeSQLite struct{handle *C.sqlite3}
func openSQLite(path string)(SQLiteDB,error){
 p:=C.CString(path);defer C.free(unsafe.Pointer(p))
 var handle *C.sqlite3
 rc:=C.sqlite3_open_v2(p,&handle,C.SQLITE_OPEN_READWRITE|C.SQLITE_OPEN_CREATE|C.SQLITE_OPEN_FULLMUTEX,nil)
 if rc!=C.SQLITE_OK {return nil,fmt.Errorf("open sqlite %d",int(rc))}
 d:=&nativeSQLite{handle}
 for _,q:=range []string{"PRAGMA foreign_keys=ON","PRAGMA busy_timeout=15000","PRAGMA journal_mode=WAL"} {if e:=d.Exec(q);e!=nil{d.Close();return nil,e}}
 return d,nil
}
func (d *nativeSQLite) Close() error{if d.handle!=nil{C.sqlite3_close_v2(d.handle);d.handle=nil};return nil}
func (d *nativeSQLite) prepare(sql string,args []any)(*C.sqlite3_stmt,error){
 q:=C.CString(sql);defer C.free(unsafe.Pointer(q))
 var stmt *C.sqlite3_stmt
 if C.sqlite3_prepare_v2(d.handle,q,-1,&stmt,nil)!=C.SQLITE_OK{return nil,fmt.Errorf("sqlite prepare: %s",C.GoString(C.sqlite3_errmsg(d.handle)))}
 for i,arg:=range args {
  idx:=C.int(i+1);var rc C.int
  switch v:=arg.(type){
  case nil:rc=C.sqlite3_bind_null(stmt,idx)
  case int:rc=C.sqlite3_bind_int64(stmt,idx,C.sqlite3_int64(v))
  case int64:rc=C.sqlite3_bind_int64(stmt,idx,C.sqlite3_int64(v))
  case float64:rc=C.sqlite3_bind_double(stmt,idx,C.double(v))
  default:sv:=fmt.Sprint(v);cs:=C.CString(sv);rc=C.sqlite3_bind_text(stmt,idx,cs,C.int(len(sv)),C.SQLITE_TRANSIENT);C.free(unsafe.Pointer(cs))
  }
  if rc!=C.SQLITE_OK{C.sqlite3_finalize(stmt);return nil,fmt.Errorf("sqlite bind: %s",C.GoString(C.sqlite3_errmsg(d.handle)))}
 }
 return stmt,nil
}
func (d *nativeSQLite) Query(sql string,args ...any)([]Row,error){
 stmt,e:=d.prepare(sql,args);if e!=nil{return nil,e};defer C.sqlite3_finalize(stmt)
 n:=int(C.sqlite3_column_count(stmt));names:=make([]string,n);for i:=range names{names[i]=C.GoString(C.sqlite3_column_name(stmt,C.int(i)))}
 rows:=make([]Row,0)
 for {
  rc:=C.sqlite3_step(stmt)
  if rc==C.SQLITE_DONE{return rows,nil}
  if rc!=C.SQLITE_ROW{return nil,fmt.Errorf("sqlite query: %s",C.GoString(C.sqlite3_errmsg(d.handle)))}
  row:=make(Row,n)
  for i,name:=range names {j:=C.int(i);typ:=C.sqlite3_column_type(stmt,j);switch typ{
   case C.SQLITE_NULL:row[name]=nil
   case C.SQLITE_INTEGER:row[name]=int64(C.sqlite3_column_int64(stmt,j))
   case C.SQLITE_FLOAT:row[name]=float64(C.sqlite3_column_double(stmt,j))
   default: p:=C.sqlite3_column_text(stmt,j);ln:=int(C.sqlite3_column_bytes(stmt,j));if ln==0{row[name]=""}else{row[name]=C.GoStringN((*C.char)(unsafe.Pointer(p)),C.int(ln))}
  }}
  rows=append(rows,row)
 }
}
func (d *nativeSQLite) Exec(sql string,args ...any)error{
 stmt,e:=d.prepare(sql,args);if e!=nil{return e};defer C.sqlite3_finalize(stmt)
 for {rc:=C.sqlite3_step(stmt);if rc==C.SQLITE_DONE{return nil};if rc!=C.SQLITE_ROW{return fmt.Errorf("sqlite exec: %s",C.GoString(C.sqlite3_errmsg(d.handle)))}}
}
