package uerr

//
// a const error type
//
//    const ErrWow = uerr.Const("wow problem")
//
type Const string

func (e Const) Error() string { return string(e) }
