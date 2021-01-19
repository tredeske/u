package uconfig

import "testing"

func TestSI(t *testing.T) {
	s := "1000"

	sv, err := Int64FromSiString(s)
	if err != nil {
		t.Fatal(err)
	} else if 1000 != sv {
		t.Fatalf("value is %d, should be 1000", sv)
	}

	s = "1k"
	sv, err = Int64FromSiString(s)
	if err != nil {
		t.Fatal(err)
	} else if 1000 != sv {
		t.Fatalf("value is %d, should be 1000", sv)
	}

	s = "1ki"
	sv, err = Int64FromSiString(s)
	if err != nil {
		t.Fatal(err)
	} else if 1024 != sv {
		t.Fatalf("value is %d, should be 1024", sv)
	}

	s = "1i"
	_, err = Int64FromSiString(s)
	if nil == err {
		t.Fatal("should have errored when converting '1i'")
	}

	s = "1.1k"
	sv, err = Int64FromSiString(s)
	if err != nil {
		t.Fatal(err)
	} else if 1100 != sv {
		t.Fatalf("value is %d, should be 1100", sv)
	}

	s = "1.1k"
	fv, err := Float64FromSiString(s)
	if err != nil {
		t.Fatal(err)
	} else if float64(1100) != fv {
		t.Fatalf("value is %f, should be 1100", fv)
	}
}
