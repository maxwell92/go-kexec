package main

import "io/ioutil"

var python27Template = `FROM python:2.7
ADD . ./
ENTRYPOINT [ "python", "exec" ]
`

func main() {
	ioutil.WriteFile("output.test", []byte(python27Template), 0644)
}
