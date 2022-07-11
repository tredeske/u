package uconfig

import (
	"fmt"
	"reflect"
)

//
// make sure types are assignable, then assign
//
//    var it interface{}
//    ...
//    var toMap map[string]bool
//    err := Assign( "context", &toMap, it )
//
func Assign(name string, dst, src interface{}) (err error) {
	dstV := reflect.ValueOf(dst)
	dstT := dstV.Type()
	if dstT.Kind() != reflect.Ptr {
		err = fmt.Errorf("target type for %s not a pointer.  is %s", name, dstT)
		return
	}
	dstV = dstV.Elem()
	dstT = dstV.Type()
	srcV := reflect.ValueOf(src)
	srcT := srcV.Type()
	if !srcT.AssignableTo(dstT) {
		err = fmt.Errorf("cannot assign %s to %s for %s", srcT, dstT, name)
		return
	}
	dstV.Set(srcV)
	return
}
