package temps

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
)

var Temps map[string]string

func init() {
	Temps = make(map[string]string)
	Temps["allEndpoint"] = allEndpoint
	Temps["endpoint"] = endpoint
	Temps["instrument"] = instrument
	Temps["request"] = request
	Temps["router"] = router
	Temps["service"] = service
	Temps["allService"] = allService
	Temps["response"] = response
	projectName, err := GetProjectName()
	if err != nil {
		panic(err)
	}
	// TODO:  这里也用模板,替换map里的
	fmt.Println("项目名称:", projectName)
	for k, v := range Temps {
		Temps[k] = strings.ReplaceAll(v, "$$$", projectName)
	}
}
func GetProjectName() (string, error) {
	all, err := ioutil.ReadFile("../go.mod")
	if err != nil {
		return "", fmt.Errorf("没找到 ../go.mod")
	}
	c := bytes.Index(all, []byte("module"))
	if c < 0 {
		return "", fmt.Errorf("gomod里没找到 " + "`module " + "`" + "有个空格哈")
	}
	d := bytes.Index(all[c:], []byte("\n"))
	if c < 0 {
		return "", fmt.Errorf("gomod里的module XXX 后面没有杠n")
	}
	return string(all[c+7 : d]), nil
}
