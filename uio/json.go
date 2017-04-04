package uio

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

//
// load the YAML file into target, which may be a ptr to map or ptr to struct
//
func YamlLoad(file string, target interface{}) error {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(content, target)
}

//
// store contents of source (a map or a struct) into file as YAML
//
func YamlStore(file string, source interface{}) (err error) {
	data, err := yaml.Marshal(source)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, data, 0664)
}

//
// store contents of source (a map or a struct) into file as JSON
//
func JsonStore(file string, it interface{}) (err error) {
	return FileCreate(file,
		func(f *os.File) error {
			return json.NewEncoder(f).Encode(it)
		})
}

//
// load the JSON file into target, which may be a ptr to map or ptr to struct
//
func JsonLoad(file string, target interface{}) (err error) {
	jsonF, err := os.Open(file)
	if nil == err {
		err = json.NewDecoder(jsonF).Decode(target)
		jsonF.Close()
	}
	return
}

//
// load the JSON file into target, which may be a ptr to map or ptr to struct
// if file does not exist, then leave target unchanged and do not error
//
func JsonLoadIfExists(file string, target interface{}) (err error) {
	err = JsonLoad(file, target)
	if err != nil {
		perr, ok := err.(*os.PathError)
		if ok && perr.Op == "open" { // ignore could not open
			err = nil
		}
	}
	return
}

//
// store contents of source (a map or a struct) into string as JSON
//
func JsonString(source interface{}) string {
	//jsonString, err := json.Marshal(source)
	bytes, err := json.MarshalIndent(source, "", " ")
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}
