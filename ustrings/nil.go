package ustrings

import "reflect"

// deal with golang nil crazyness for interfaces
func ItIsNil(it any) bool {
	if nil == it {
		return true
	}
	v := reflect.ValueOf(it)
	k := v.Kind()
	return (k == reflect.Ptr ||
		k == reflect.Map ||
		k == reflect.Slice ||
		k == reflect.Chan ||
		k == reflect.Func ||
		k == reflect.Interface) && v.IsNil()
}
