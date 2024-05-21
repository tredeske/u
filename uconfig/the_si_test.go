package uconfig

import "testing"

func TestSI(t *testing.T) {
	s := "1000"

	var sv int64
	err := IntFromSiString(s, &sv)
	if err != nil {
		t.Fatal(err)
	} else if 1000 != sv {
		t.Fatalf("value is %d, should be 1000", sv)
	}

	s = "1k"
	err = IntFromSiString(s, &sv)
	if err != nil {
		t.Fatal(err)
	} else if 1000 != sv {
		t.Fatalf("value is %d, should be 1000", sv)
	}

	s = "1k"
	err = IntFromBitRateString(s, &sv)
	if err != nil {
		t.Fatal(err)
	} else if 1000 != sv {
		t.Fatalf("value is %d, should be 1000", sv)
	}

	s = "1k"
	err = IntFromByteSizeString(s, &sv)
	if err != nil {
		t.Fatal(err)
	} else if 1024 != sv { // note
		t.Fatalf("value is %d, should be 1024", sv)
	}

	s = "1ki"
	err = IntFromSiString(s, &sv)
	if err != nil {
		t.Fatal(err)
	} else if 1024 != sv {
		t.Fatalf("value is %d, should be 1024", sv)
	}

	s = "1ki"
	err = IntFromByteSizeString(s, &sv)
	if err != nil {
		t.Fatal(err)
	} else if 1024 != sv {
		t.Fatalf("value is %d, should be 1024", sv)
	}

	s = "1ki"
	err = IntFromBitRateString(s, &sv)
	if nil == err {
		t.Fatalf("Expected failure")
	}

	s = "1i"
	err = IntFromSiString(s, &sv)
	if nil == err {
		t.Fatal("should have errored when converting '1i'")
	}

	s = "1.1k"
	err = IntFromSiString(s, &sv)
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
