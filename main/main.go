package main

import (
	"bytes"
	"fmt"
	"genarate/temps"
	u "genarate/utils"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type templateParams struct {
	p map[string]string
	e interface{}
}

var a = map[bool]string{true: "找到了", false: "没找到"}

type NameFunc struct {
	Need string
	F    func(string) string
}

type nodeWithCancel struct {
	HasCancel bool
	FuncNode  *dst.FuncDecl
}

func GetRequest5(funcName string) (m map[string]*u.KeyNodePosition, err error) {
	params := map[string]string{
		"FuncName": funcName,
	}
	for k, v := range params {
		if v == "" {
			params[k] = "Xx"
		}
	}
	templatePath := "request"
	nodeMap, err := u.GetKeyNodeFromTemplateMap(templatePath, params,
		filepath.Join(`./node_dst`, u.GetFileNamePure(templatePath)), nil, "req1")
	if err != nil {
		log.Panicln(err)
	}
	return nodeMap, err
}

// topic默认是service_topic.go,当蛇形处理
func main() {
	write2FileSlice := make([]*InsertDstFileV2, 0, 8) //需要更新的文件路径及其语法树

	serviceDir := "../internal/services/"
	servicePackage, _, _, err := u.ParseDir(serviceDir, func(info os.FileInfo) bool {
		compile, err := regexp.Compile(`service_.*`)
		if err != nil {
			log.Println(err)
		}
		m := compile.MatchString(info.Name())
		return m
	})
	if err != nil {
		log.Panicln(err)
	}

	topicUrlMap := make(map[string]map[string]*nodeWithCancel) // topic:url:node
	for filePath, fileDst := range servicePackage.Files {      // filePath 是全路径
		funcNodes := make([]*dst.FuncDecl, 0, 2)              //用于serviceInterface的添加
		fileNamePure := u.GetFileNamePure(filePath)           // fileNamePure=service_contact ,文件名不带后缀
		topic := strings.TrimPrefix(fileNamePure, "service_") // topic=contact, 一个文件代表一个主题
		topic = u.Snack2SmallCamelCase(topic)                 //进行蛇形转小驼峰
		routerFlag := `//@router `
		routerFinishFlag := `//@decode `
		_ = u.DeepApply(fileDst, func(cursor *dstutil.Cursor, c *u.Deep) {
			if c.Deep() > 1 { //只遍历 file和顶层声明 file.Decl
				c.MovePan()
				return
			}
			node := cursor.Node()
		breakSwitch:
			switch n := node.(type) {
			case *dst.FuncDecl:
				if Deco := n.Decorations(); Deco != nil {
					var url string
					var getRouterFlag bool //需要这个的原因是,url==""没有作为判断依据,允许路由是空
					for _, j := range Deco.Start {
						if strings.HasPrefix(j, routerFinishFlag) { //如果有完成标志,就不要再遍历注释了,直接下一个节点
							break breakSwitch
						}
						if strings.HasPrefix(j, routerFlag) { //注释是否有 `//router `前缀
							url = j[len(routerFlag):] // url就是注释`//router /a/save` 的  /a/save
							getRouterFlag = true
						}
					}
					if !getRouterFlag {
						break
					}
					funcNodes = append(funcNodes, n)
					funcName := n.Name.Name
					// 添加一个注释,形如 `//decode contract.DecodeContractSaveRequest`  便于直接跳转到解析请求的函数
					Deco.Start.Append(`//@decode ` + topic + `.Decode` + funcName + `Request`)
					if _, ok := topicUrlMap[topic]; !ok { //第一次需要初始化
						topicUrlMap[topic] = make(map[string]*nodeWithCancel)
					}
					if _, ok := topicUrlMap[topic][url]; ok {
						log.Panicf("文件路径:%s,重复的url地址:%s", filePath, url)
					}
					topicUrlMap[topic][url] = &nodeWithCancel{
						FuncNode: n,
					}
				}
			}
		})
		if _, ok := topicUrlMap[topic]; ok {
			// 由于这里检查 @Router注解时,会追加一个完成标志,所以要传入这个service的fileDst
			file, _, err := service5(topic, funcNodes, fileDst) //增加interface的代码
			if err != nil {
				panic(err)
			}
			//write2FileSlice = append(write2FileSlice, NewInsertDstFileV2(fileDst, filePath))
			write2FileSlice = append(write2FileSlice, file)
		}
	}
	if len(topicUrlMap) == 0 {
		return
	}

	instrumentParam := make([]*templateParams, 0, 8)
	allServiceParam := make([]*templateParams, 0, 8)
	allEndpointParam := make([]*templateParams, 0, 8)
	requestParam := make([]*templateParams, 0, 8)
	responseParam := make([]*templateParams, 0, 8)
	for topic, urlNodeMap := range topicUrlMap {
		allServiceParam = append(allServiceParam, &templateParams{
			p: map[string]string{
				"Topic": topic,
			}})

		urlSlice := make([]string, 0, len(urlNodeMap))
		endpointParam := make([]*templateParams, 0, len(urlNodeMap))
		routerParam := make([]*templateParams, 0, len(urlNodeMap))
		for url, node := range urlNodeMap {
			urlSlice = append(urlSlice, url)
			funcName := node.FuncNode.Name.Name
			endpointParam = append(endpointParam, &templateParams{
				p: map[string]string{
					"FuncName": funcName,
					"Topic":    topic,
				}})
			instrumentParam = append(instrumentParam, &templateParams{
				p: map[string]string{
					"FuncName": funcName,
				}})
			routerParam = append(routerParam, &templateParams{
				p: map[string]string{
					"FuncName": funcName,
					"Url":      url,
					"Topic":    topic,
				}})
			allEndpointParam = append(allEndpointParam, &templateParams{
				p: map[string]string{
					"FuncName": funcName,
				},
			})
			requestParam = append(requestParam, &templateParams{
				p: map[string]string{
					"FuncName": funcName,
				},
			})
			responseParam = append(responseParam, &templateParams{
				p: map[string]string{
					"FuncName": funcName,
				},
			})

		}

		endpointDstFile, _, err := endpoint5(topic, endpointParam)
		if err != nil {
			panic(err)
		}
		write2FileSlice = append(write2FileSlice, endpointDstFile)

		routerDstFile, _, err := router5(topic, routerParam, urlNodeMap)
		if err != nil {
			panic(err)
		}
		write2FileSlice = append(write2FileSlice, routerDstFile)

	}

	responseDstFile, _, err := response5(responseParam)
	if err != nil {
		panic(err)
	}
	write2FileSlice = append(write2FileSlice, responseDstFile)

	requestDstFile, _, err := request5(requestParam)
	if err != nil {
		panic(err)
	}
	write2FileSlice = append(write2FileSlice, requestDstFile)

	allEndpointDstFile, _, err := allEndpoint5(allEndpointParam)
	if err != nil {
		panic(err)
	}
	write2FileSlice = append(write2FileSlice, allEndpointDstFile)

	allServiceDstFile, _, err := allService5(allServiceParam, nil)
	if err != nil {
		panic(err)
	}
	write2FileSlice = append(write2FileSlice, allServiceDstFile)

	instrumentDstFile, _, err := instrument5(instrumentParam, nil)
	if err != nil {
		panic(err)
	}
	write2FileSlice = append(write2FileSlice, instrumentDstFile)

	//===============================================================
	// 将语法树恢复到文件中
	// 1.路由
	// 2.service的注解
	checkPath := make(map[string]struct{}, len(write2FileSlice))
	for _, j := range write2FileSlice {
		if _, ok := checkPath[j.FilePath]; ok {
			panic(fmt.Errorf("在对同一个文件进行多次写入dst"))
		}
		err := j.CheckSrc()
		//err := j.Rewrite()
		if err != nil {
			panic(fmt.Errorf("语法树有错误,filePath:%v,err:%v", j.FilePath, err))
		}
	}
	for _, j := range write2FileSlice {
		err := j.Src2File()
		if err != nil {
			panic(fmt.Errorf("写入文件有错误,filePath:%v,err:%v", j.FilePath, err))
		}
	}
}

func response5(params []*templateParams) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*templateParams) error, err error) {
	if len(params) == 0 {
		return nil, nil, fmt.Errorf("router 参数为空")
	}
	realParamsTrans := map[string]*NameFunc{ //这是模板需要的
		"FuncName": {Need: "FuncName", F: nil},
	}
	realParams := make(map[string]string, len(realParamsTrans))

	var t = "response"
	var realSlice []*templateParams
	//var templatePath = "../template/router.txt"
	var dir = `../internal/proto`
	var pathFunc = func() string {
		return filepath.Join(dir, "response.go")
	}
	filePath := pathFunc()
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf(t+".  filePath:%v>>>%s\n", filePath, a[hasExist])

	if !hasExist {
		realSlice = params[1:]
		for name, trans := range realParamsTrans {
			if vv, ok := params[0].p[trans.Need]; ok {
				if trans.F != nil {
					realParams[name] = trans.F(vv)
				} else {
					realParams[name] = vv
				}
				continue
			}
			err = fmt.Errorf("传入模板的参数错误")
			return
		}
		dstFile, err = NewInsertDstFileV2FromTemplateMap(t, realParams, filePath,
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, nil)
	} else {
		realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
	}
	if err != nil {
		return
	}
	dstFile.InsertFunc["p1"] = u.InsertFloat(dstFile.Tree, "Decls", len(dstFile.Tree.Decls)-1, true)

	checkAndInsertFunc = func(paramsSlice []*templateParams) error {
		for _, j := range paramsSlice {
			// 处理模板需要的参数
			for name, trans := range realParamsTrans {
				if vv, ok := j.p[trans.Need]; ok {
					if trans.F != nil {
						realParams[name] = trans.F(vv)
					} else {
						realParams[name] = vv
					}
					continue
				}
				return fmt.Errorf("传入模板的参数错误")
			}

			nodeMap, err := u.GetKeyNodeFromTemplateMap(t, realParams,
				filepath.Join(`./node_dst`, t, t+"_temp"+time.Now().Format("02150405")),
				nil, "k1")
			if err != nil {
				return err
			}
			err = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
				Position: "p1",
				Keys:     []string{"k1"},
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = checkAndInsertFunc(realSlice)
	return
}

func request5(params []*templateParams) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*templateParams) error, err error) {
	if len(params) == 0 {
		return nil, nil, fmt.Errorf("request 参数为空")
	}
	realParamsTrans := map[string]*NameFunc{ //这是模板需要的
		"FuncName": {Need: "FuncName", F: nil},
	}
	realParams := make(map[string]string, len(realParamsTrans))

	var t = "request"
	var realSlice []*templateParams
	//var templatePath = "../template/router.txt"
	var dir = `../internal/proto`
	var pathFunc = func() string {
		return filepath.Join(dir, "request.go")
	}
	filePath := pathFunc()
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf(t+".  filePath:%v>>>%s\n", filePath, a[hasExist])

	if !hasExist {
		realSlice = params[1:]
		for name, trans := range realParamsTrans {
			if vv, ok := params[0].p[trans.Need]; ok {
				if trans.F != nil {
					realParams[name] = trans.F(vv)
				} else {
					realParams[name] = vv
				}
				continue
			}
			err = fmt.Errorf("传入模板的参数错误")
			return
		}
		dstFile, err = NewInsertDstFileV2FromTemplateMap(t, realParams, filePath,
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, nil)
	} else {
		realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
	}
	if err != nil {
		return
	}
	dstFile.InsertFunc["p1"] = u.InsertFloat(dstFile.Tree, "Decls", len(dstFile.Tree.Decls)-1, true)

	checkAndInsertFunc = func(paramsSlice []*templateParams) error {
		for _, j := range paramsSlice {
			// 处理模板需要的参数
			for name, trans := range realParamsTrans {
				if vv, ok := j.p[trans.Need]; ok {
					if trans.F != nil {
						realParams[name] = trans.F(vv)
					} else {
						realParams[name] = vv
					}
					continue
				}
				return fmt.Errorf("传入模板的参数错误")
			}

			nodeMap, err := u.GetKeyNodeFromTemplateMap(t, realParams,
				filepath.Join(`./node_dst`, t, t+"_temp"+time.Now().Format("02150405")),
				nil, "req1")
			if err != nil {
				return err
			}
			err = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
				Position: "p1",
				Keys:     []string{"req1"},
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = checkAndInsertFunc(realSlice)
	return
}

func service5(topic string, extra interface{}, dFile *dst.File) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*dst.FuncDecl) error, err error) {
	funcNodes, ok := extra.([]*dst.FuncDecl)
	if !ok || len(funcNodes) == 0 {
		return nil, nil, fmt.Errorf("service()参数错误")
	}

	for i := range funcNodes {
		if funcNodes[i] == nil {
			return nil, nil, fmt.Errorf("service()参数有nil")
		}
	}

	var t = "service"
	//var templatePath = "../template/" + t + ".txt"
	var dir = `../internal/services`
	var pathFunc = func(topic string) string {
		return filepath.Join(dir, "service_"+u.Camel2SnackCase(topic)+".go")
	}
	filePath := pathFunc(topic)
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf("service.  filePath:%v>>>%s\n", filePath, a[hasExist])

	if !hasExist {
		return nil, nil, fmt.Errorf("service路径转化错误")
		//realSlice = params[1:]
		//for name, trans := range realParamsTrans {
		//	if vv, ok := params[0].p[trans.Need]; ok {
		//		if trans.F != nil {
		//			realParams[name] = trans.F(vv)
		//		} else {
		//			realParams[name] = vv
		//		}
		//		continue
		//	}
		//	err = fmt.Errorf("传入模板的参数错误")
		//	return
		//}
		//dstFile, err = NewInsertDstFileV2FromTemplate(templatePath, realParams, filePath,
		//	filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
		//	nil, nil)
	} else {
		//realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
		if err != nil {
			return
		}
		dstFile.Tree = dFile //在外界修改了树,所以就用外界的
	}

	//===============================================================
	// 1.先判断对应的接口和结构体存在否,不存在就插入一个, 并需要定义插入的位置
	var hasFindIser bool
	var hasFindImport bool
	var iSerName = "I" + u.FirstBigCase(topic) + "Service"
	var check1 *dst.InterfaceType
	err = u.DeepApply(dstFile.Tree, func(cursor *dstutil.Cursor, c *u.Deep) {
		if c.Deep() > 1 { //只会遍历顶层声明
			c.MovePan()
			return
		}
		if hasFindIser {
			c.Break(nil)
		}
		node := cursor.Node()
		switch n := node.(type) {
		case *dst.GenDecl:
			if n.Tok == token.IMPORT {
				dstFile.InsertFunc["top"] = u.InsertFloatFromCursor(cursor.Parent(), cursor.Name(), cursor.Index(), true)
				hasFindImport = true
				c.MovePan()
				return
			}
			if len(n.Specs) == 0 {
				c.MovePan()
				return
			}
			s, ok := n.Specs[0].(*dst.TypeSpec)
			if ok && s.Name.Name == iSerName {
				hasFindIser = true
				itface := s.Type.(*dst.InterfaceType)
				check1 = itface
				dstFile.InsertFunc["iser"] = u.InsertFixed(itface.Methods, "List", 0)
			}
		}
	})
	// 2.如果没有找到import
	fmt.Printf("service->import>>>%v\n", a[hasFindImport])
	keys := []string{"k1", "k2"}
	if !hasFindImport {
		dstFile.InsertFunc["top"] = u.InsertFloat(dstFile.Tree, "Decls", 0)
		keys = append(keys[1:], keys...)
		keys[0] = "k0" //k0是import
	}
	// 3.如果没有找到serviceInterface
	fmt.Printf("service->接口:%v>>>%v\n", iSerName, a[hasFindIser])
	if !hasFindIser {
		keysNode, err := u.GetKeyNodeFromTemplateMap(t,
			map[string]string{"TopicBig": u.FirstBigCase(topic), "FuncName": "GG"},
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, "k0", "k1", "k2")
		if err != nil {
			return nil, nil, err
		}
		err = dstFile.InsertFromMapV2(keysNode, &PositionAndKey{
			Position: "top",
			Keys:     keys,
		})
		if err != nil {
			return nil, nil, err
		}
		// 4.因为没找到serviceInterface,要设定插入函数
		k1 := keysNode["k1"]
		check1 = k1.Self.(*dst.GenDecl).Specs[0].(*dst.TypeSpec).Type.(*dst.InterfaceType)
		dstFile.InsertFunc["iser"] = u.InsertFixed(check1.Methods, "List", 0)
	}

	checkAndInsertFunc = func(paramsSlice []*dst.FuncDecl) error {
		for _, fNode := range paramsSlice {
			// 1.将对应的service方法改为标准格式
			funcName := fNode.Name.Name
			keysNodes, err := u.GetKeyNodeFromTemplateMap(t,
				map[string]string{"TopicBig": u.FirstBigCase(topic), "FuncName": funcName},
				filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
				nil, "k4", "k5")
			if err != nil {
				return err
			}
			decl := dst.Clone(keysNodes["k4"].Self).(*dst.FuncDecl)
			fNode.Recv = decl.Recv
			fNode.Type.Params = decl.Type.Params
			fNode.Type.Results = decl.Type.Results
			// 1.2 追加 return nil,nil
			if len(fNode.Body.List) == 0 {
				if fNode.Body.List == nil {
					fNode.Body.List = make([]dst.Stmt, 0)
				}
				fNode.Body.List = append(fNode.Body.List, keysNodes["k5"].Self.(*dst.ReturnStmt))
			}
			// 2.添加interface部分的代码
			// 检查冲突
			var hasFind bool
			for _, field := range check1.Methods.List {
				if len(field.Names) == 0 {
					continue
				}
				// TODO:  没有做nil检查
				if field.Names[0].Name == fNode.Name.Name { //比较对应的方法是否已经添加到了interface
					hasFind = true
				}
			}
			if hasFind {
				continue
			}
			// 如果没找到,就拷贝funcDecl的声明的类型和名称,即Type和Name,组装一下放到interface的filed
			var field = new(dst.Field)
			field.Names = make([]*dst.Ident, 1)
			field.Names[0] = dst.Clone(fNode.Name).(*dst.Ident)
			field.Type = dst.Clone(fNode.Type).(*dst.FuncType)
			err = dstFile.Insert("iser", field)
			if err != nil {
				return err
			}

		}
		return nil
	}
	err = checkAndInsertFunc(funcNodes)
	return
}

// 较为标准
func allService5(params []*templateParams, extra interface{}) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*templateParams) error, err error) {
	if len(params) == 0 {
		return nil, nil, fmt.Errorf("allservice 参数为空")
	}
	realParamsTrans := map[string]*NameFunc{ //这是模板需要的
		"Topic": {Need: "Topic", F: u.FirstBigCase},
	}
	realParams := make(map[string]string, len(realParamsTrans))

	var realSlice []*templateParams
	var t = "allService"
	//var templatePath = "../template/" + t + ".txt"
	var dir = `../internal/services`
	var pathFunc = func() string {
		return filepath.Join(dir, "service.go")
	}
	filePath := pathFunc()
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf("allService.  filePath:%v>>>%s\n", filePath, a[hasExist])
	if !hasExist {
		realSlice = params[1:]
		for name, trans := range realParamsTrans {
			if vv, ok := params[0].p[trans.Need]; ok {
				if trans.F != nil {
					realParams[name] = trans.F(vv)
				} else {
					realParams[name] = vv
				}
				continue
			}
			err = fmt.Errorf("传入模板的参数错误")
			return
		}
		dstFile, err = NewInsertDstFileV2FromTemplateMap(t, realParams, filePath,
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, nil)
	} else {
		realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
	}
	if err != nil {
		return
	}
	//===============================================================
	// 找到插入点
	var check1 *dst.InterfaceType
	var check2 *dst.StructType
	var hasFindIService bool
	var hasFindService bool
	_ = u.DeepApply(dstFile.Tree, func(cursor *dstutil.Cursor, c *u.Deep) {
		if c.Deep() > 1 { //没找到目标之前,只会遍历顶层声明
			c.MovePan()
			return
		}
		if hasFindService && hasFindIService {
			c.Break(nil)
		}
		node := cursor.Node()
		switch n := node.(type) {
		case *dst.GenDecl:
			if len(n.Specs) == 0 {
				c.MovePan()
				return
			}
			s, ok := n.Specs[0].(*dst.TypeSpec)
			if ok && s.Name.Name == "IService" {
				e := s.Type.(*dst.InterfaceType)
				check1 = e //记录这个点,以便校验
				hasFindIService = true

				iser := u.InsertFloat(e.Methods, "List", len(e.Methods.List)-1, true)
				dstFile.InsertFunc["iser"] = iser //自己传一个insert函数
			}
			if ok && s.Name.Name == "Service" {
				e := s.Type.(*dst.StructType)
				check2 = e //记录这个点,以便校验
				hasFindService = true
				// 每次的最后插入
				ser := u.InsertFloat(e.Fields, "List", len(e.Fields.List)-1, true)
				dstFile.InsertFunc["ser"] = ser //自己传一个insert函数
			}
		}
	})
	fmt.Printf("allService.  IService接口>>>%s\n", a[hasFindIService])
	fmt.Printf("allService.  Service结构体>>>%s\n", a[hasFindService])
	if !hasFindIService || !hasFindService {
		return nil, nil, fmt.Errorf("IService接口或者Service结构体没找到")
	}

	// 检查和插入的函数
	checkAndInsertFunc = func(paramsSlice []*templateParams) (err1 error) {
		for _, topicM := range paramsSlice {
			topicBig := u.FirstBigCase(topicM.p["Topic"])

			//===============================================================
			// 校验存在否
			iService := "I" + topicBig + "Service"
			service := topicBig + "Service"
			var hasFindIServiceConflict bool
			var hasFindServiceConflict bool
			for _, field := range check1.Methods.List {
				var name string
				switch t := field.Type.(type) { //interface下,可能是匿名接口或方法
				case *dst.Ident:
					name = t.Name
				case *dst.FuncType:
					name = field.Names[0].Name
				}
				if iService == name {
					hasFindIServiceConflict = true
				}
			}
			for _, field := range check2.Fields.List {
				if service == field.Type.(*dst.Ident).Name {
					hasFindServiceConflict = true
				}
			}

			fmt.Printf("allService.  IService.Field.  %v>>>%s\n", iService, a[hasFindIServiceConflict])
			fmt.Printf("allService.  Service.Field.  %v>>>%s\n", service, a[hasFindServiceConflict])
			if hasFindIServiceConflict && hasFindServiceConflict {
				continue
			}

			//===============================================================
			// 处理模板需要的参数,并插入
			for name, trans := range realParamsTrans {
				if vv, ok := topicM.p[trans.Need]; ok {
					if trans.F != nil {
						realParams[name] = trans.F(vv)
					} else {
						realParams[name] = vv
					}
					continue
				}
				err = fmt.Errorf("传入模板的参数错误")
				return
			}
			nodeMap, err2 := u.GetKeyNodeFromTemplateMap(t, realParams,
				filepath.Join(`./node_dst`, t, t+"_temp"+time.Now().Format("02150405")),
				nil, "k1", "k2")
			if err2 != nil {
				return err2
			}
			if !hasFindIServiceConflict {
				err1 = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
					Position: "iser",
					Keys:     []string{"k1"},
				})
				if err1 != nil {
					return err1
				}
			}
			if !hasFindIServiceConflict {
				err1 = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
					Position: "ser",
					Keys:     []string{"k2"},
				})
				if err1 != nil {
					return err1
				}
			}
		}
		return
	}
	err = checkAndInsertFunc(realSlice)
	return
}

func router5(topic string, params []*templateParams, extra interface{}) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*templateParams) error, err error) {
	if len(params) == 0 {
		return nil, nil, fmt.Errorf("router 参数为空")
	}
	realParamsTrans := map[string]*NameFunc{ //这是模板需要的
		"FuncName": {Need: "FuncName", F: nil},
		"Url":      {Need: "Url", F: nil},
		"TopicBig": {Need: "Topic", F: u.FirstBigCase},
		"Package":  {Need: "Topic", F: nil},
	}
	realParams := make(map[string]string, len(realParamsTrans))

	var t = "router"
	var realSlice []*templateParams
	//var templatePath = "../template/router.txt"
	var dir = `../internal/transports`
	var pathFunc = func(topic string) string {
		return filepath.Join(dir, topic, "router.go")
	}
	filePath := pathFunc(topic)
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf("allService.  filePath:%v>>>%s\n", filePath, a[hasExist])

	if !hasExist {
		realSlice = params[1:]
		for name, trans := range realParamsTrans {
			if vv, ok := params[0].p[trans.Need]; ok {
				if trans.F != nil {
					realParams[name] = trans.F(vv)
				} else {
					realParams[name] = vv
				}
				continue
			}
			err = fmt.Errorf("传入模板的参数错误")
			return
		}
		dstFile, err = NewInsertDstFileV2FromTemplateMap(t, realParams, filePath,
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, nil)
	} else {
		realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
		if err != nil {
			return
		}
		//===============================================================
		m := extra.(map[string]*nodeWithCancel)
		var hasFindLoad bool
		err = u.DeepApply(dstFile.Tree, func(cursor *dstutil.Cursor, c *u.Deep) {
			if c.Deep() > 1 && !hasFindLoad { //没找到LoadFunc之前,只会遍历顶层声明
				c.MovePan()
				return
			}
			node := cursor.Node()
			switch n := node.(type) {
			case *dst.FuncDecl:
				if strings.HasPrefix(n.Name.Name, "Load") { //只要是Load开头的函数就认定为路由
					hasFindLoad = true
				}
			case *dst.CallExpr:
				if len(n.Args) == 0 {
					c.MovePan()
					return
				}
				lit, ok := n.Args[0].(*dst.BasicLit)
				if !ok {
					c.MovePan()
					return
				}
				existUrl, err := strconv.Unquote(lit.Value)
				if err != nil {
					c.Break(err)
				}
				if _, ok := m[existUrl]; ok { //扫描service得到的urlMap都是需要生成的,不能有冲突
					err = fmt.Errorf(topic+"的url冲突: %s", existUrl)
					c.Break(err)
				}
			}
		})
		if !hasFindLoad {
			return nil, nil, fmt.Errorf("没找到Load函数,topic:%s", topic)
		}
	}
	if err != nil {
		return
	}
	_ = u.DeepApply(dstFile.Tree, func(cursor *dstutil.Cursor, c *u.Deep) {
		if c.Deep() > 1 {
			c.MovePan()
			return
		}
		node := cursor.Node()
		switch n := node.(type) {
		case *dst.FuncDecl:
			if strings.HasPrefix(n.Name.Name, "Load") {
				// 每次在最后插入
				p2 := u.InsertFloat(n.Body, "List", len(n.Body.List)-1, true)
				// 在当前节点的后面插入
				p3 := u.InsertFixedFromCursor(cursor.Parent(), cursor.Name(), cursor.Index(), true)
				dstFile.InsertFunc["p2"] = p2 //自己传一个insert函数
				dstFile.InsertFunc["p3"] = p3
				c.Break(nil)
			}
		}
	})

	checkAndInsertFunc = func(paramsSlice []*templateParams) error {
		for _, j := range paramsSlice {
			// 处理模板需要的参数
			for name, trans := range realParamsTrans {
				if vv, ok := j.p[trans.Need]; ok {
					if trans.F != nil {
						realParams[name] = trans.F(vv)
					} else {
						realParams[name] = vv
					}
					continue
				}
				return fmt.Errorf("传入模板的参数错误")
			}

			nodeMap, err := u.GetKeyNodeFromTemplateMap(t, realParams,
				filepath.Join(`./node_dst`, "router", "router"+"_temp"+time.Now().Format("02150405")),
				nil, "k1")
			if err != nil {
				return err
			}
			err = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
				Position: "p2",
				Keys:     []string{"k2"},
			}, &PositionAndKey{
				Position: "p3",
				Keys:     []string{"k3"},
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = checkAndInsertFunc(realSlice)
	return
}

func instrument5(params []*templateParams, extra interface{}) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*templateParams) error, err error) {
	if len(params) == 0 {
		return nil, nil, fmt.Errorf("instrument 参数为空")
	}
	//need := []string{ //这是调用这个函数的入参
	//	"FuncName",
	//}
	realParamsTrans := map[string]*NameFunc{ //这是模板需要的
		"FuncName": {Need: "FuncName", F: nil},
	}
	realParams := make(map[string]string, len(realParamsTrans))

	var t = "instrument"
	var realSlice []*templateParams
	//var templatePath = "../template/" + t + ".txt"
	var dir = `../internal/services`
	var pathFunc = func() string {
		return filepath.Join(dir, "instrument.go")
	}
	filePath := pathFunc()
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf("instrument.  filePath:%v>>>%s\n", filePath, a[hasExist])
	if !hasExist {
		realSlice = params[1:]
		for name, trans := range realParamsTrans {
			if vv, ok := params[0].p[trans.Need]; ok {
				if trans.F != nil {
					realParams[name] = trans.F(vv)
				} else {
					realParams[name] = vv
				}
				continue
			}
			err = fmt.Errorf("传入模板的参数错误")
			return
		}
		dstFile, err = NewInsertDstFileV2FromTemplateMap(t, realParams, filePath,
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, nil)
	} else {
		realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
	}
	if err != nil || len(realSlice) == 0 {
		return
	}
	// 每次在最后插入
	before := u.InsertFloat(dstFile.Tree, "Decls", len(dstFile.Tree.Decls)-1, true)
	dstFile.InsertFunc["p1"] = before //自己传一个insert函数

	//===============================================================
	checkAndInsertFunc = func(paramsSlice []*templateParams) error {
		for _, j := range paramsSlice {
			// 处理模板需要的参数
			for name, trans := range realParamsTrans {
				if vv, ok := j.p[trans.Need]; ok {
					if trans.F != nil {
						realParams[name] = trans.F(vv)
					} else {
						realParams[name] = vv
					}
					continue
				}
				return fmt.Errorf("传入模板的参数错误")
			}

			nodeMap, err := u.GetKeyNodeFromTemplateMap(t, realParams,
				filepath.Join(`./node_dst`, t, t+"_temp"+time.Now().Format("02150405")),
				nil, "k1")
			if err != nil {
				return err
			}
			err = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
				Position: "p1",
				Keys:     []string{"k1"},
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = checkAndInsertFunc(realSlice)
	return
}

func endpoint5(topic string, params []*templateParams) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*templateParams) error, err error) {
	if len(params) == 0 {
		return nil, nil, fmt.Errorf("endpoint 参数为空")
	}
	realParamsTrans := map[string]*NameFunc{ //这是模板需要的
		"FuncName": {Need: "FuncName", F: nil},
		"Topic":    {Need: "Topic", F: u.FirstBigCase},
	}
	realParams := make(map[string]string, len(realParamsTrans))

	var t = "endpoint"
	var realSlice []*templateParams
	//var templatePath = "../template/endpoint.txt"
	var endpointDir = `../internal/endpoints/`
	var endpointPathFunc = func(topic string) string {
		return filepath.Join(endpointDir, "endpoint_"+topic+".go")
	}
	filePath := endpointPathFunc(topic)
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf("instrument.  filePath:%v>>>%s\n", filePath, a[hasExist])
	if !hasExist {
		realSlice = params[1:]
		for name, trans := range realParamsTrans {
			if vv, ok := params[0].p[trans.Need]; ok {
				if trans.F != nil {
					realParams[name] = trans.F(vv)
				} else {
					realParams[name] = vv
				}
				continue
			}
			err = fmt.Errorf("传入模板的参数错误")
			return
		}
		dstFile, err = NewInsertDstFileV2FromTemplateMap(t, realParams, filePath,
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, nil)
	} else {
		realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
	}
	if err != nil {
		return
	}
	// 每次在最后插入
	dstFile.InsertFunc["p1"] = u.InsertFloat(dstFile.Tree, "Decls", len(dstFile.Tree.Decls)-1, true)
	checkAndInsertFunc = func(paramsSlice []*templateParams) error {
		for _, j := range paramsSlice {
			// 处理模板需要的参数
			for name, trans := range realParamsTrans {
				if vv, ok := j.p[trans.Need]; ok {
					if trans.F != nil {
						realParams[name] = trans.F(vv)
					} else {
						realParams[name] = vv
					}
					continue
				}
				return fmt.Errorf("传入模板的参数错误")
			}
			// TODO:  改为不用标志点
			nodeMap, err := u.GetKeyNodeFromTemplateMap(t, realParams,
				filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
				nil, "k1")
			if err != nil {
				return err
			}
			err = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
				Position: "p1",
				Keys:     []string{"k1"},
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = checkAndInsertFunc(realSlice)
	return
}
func allEndpoint5(params []*templateParams) (dstFile *InsertDstFileV2,
	checkAndInsertFunc func(params []*templateParams) error, err error) {
	if len(params) == 0 {
		return nil, nil, fmt.Errorf("allEndpoint 参数为空")
	}
	realParamsTrans := map[string]*NameFunc{ //这是模板需要的
		"FuncName":      {Need: "FuncName", F: nil},
		"FuncNameSmall": {Need: "FuncName", F: u.FirstSmallCase},
		"FuncNameSnack": {Need: "FuncName", F: u.Camel2SnackCase},
	}
	realParams := make(map[string]string, len(realParamsTrans))
	var realSlice []*templateParams

	var t = "allEndpoint"
	//var templatePath = "../template/" + t + ".txt"
	var dir = `../internal/endpoints`
	var pathFunc = func() string {
		return filepath.Join(dir, "endpoint.go")
	}
	filePath := pathFunc()
	hasExist, err := u.PathExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("判断路径不存在出错:%v", err)
	}
	fmt.Printf(t+".  filePath:%v>>>%s\n", filePath, a[hasExist])
	if !hasExist {
		realSlice = params[1:]
		for name, trans := range realParamsTrans {
			if vv, ok := params[0].p[trans.Need]; ok {
				if trans.F != nil {
					realParams[name] = trans.F(vv)
				} else {
					realParams[name] = vv
				}
				continue
			}
			err = fmt.Errorf("传入模板的参数错误")
			return
		}
		dstFile, err = NewInsertDstFileV2FromTemplateMap(t, realParams, filePath,
			filepath.Join("./node_dst", t, t+"_temp"+time.Now().Format("02150405")),
			nil, nil)
	} else {
		realSlice = params
		dstFile, err = NewInsertDstFileV2FromFile(filePath, nil,
			filepath.Join("./node_dst", t, t+"_src"+time.Now().Format("02150405")),
			nil, nil)
	}
	if err != nil {
		return
	}
	//===============================================================
	// 找到插入点
	//var check1 *dst.InterfaceType
	//var check2 *dst.StructType
	var hasFindMakeAll bool
	var hasFindAllEndpoint bool
	var hasFindEps bool

	err = u.DeepApply(dstFile.Tree, func(cursor *dstutil.Cursor, c *u.Deep) {
		if hasFindMakeAll && hasFindAllEndpoint {
			c.Break(nil)
		}
		if c.Deep() > 1 { //没找到目标之前,只会遍历顶层声明
			c.MovePan()
			return
		}
		node := cursor.Node()
		switch n := node.(type) {
		case *dst.GenDecl:
			if len(n.Specs) == 0 {
				c.MovePan()
				return
			}
			s, ok := n.Specs[0].(*dst.TypeSpec)
			if !ok || s.Name.Name != "AllEndpoint" {
				c.MovePan()
				return
			}
			hasFindAllEndpoint = true
			// 插入点
			e := s.Type.(*dst.StructType)
			all := u.InsertFixed(e.Fields, "List", 0)
			dstFile.InsertFunc["all"] = all
		case *dst.FuncDecl:
			if n.Name.Name != "MakeAllEndpoint" {
				c.MovePan()
				return
			}
			hasFindMakeAll = true
			// 找eps这个点
			var epsNode *dst.AssignStmt
			var epsNodeIndex = 0
			_ = u.DeepApply(n.Body, func(cursor1 *dstutil.Cursor, c1 *u.Deep) {
				if hasFindEps {
					c1.Break(nil)
				}
				if c1.Deep() > 2 {
					c1.MovePan()
					return
				}
				switch n1 := cursor1.Node().(type) {
				case *dst.AssignStmt:
					epsNode = n1
					epsNodeIndex = cursor1.Index()
				case *dst.Ident:
					if n1.Name != "eps" {
						c1.MovePan()
						return
					}
					hasFindEps = true
					c1.Break(nil)
				}

			})
			if hasFindEps {
				// 插入点
				// 每次在eps之前插入
				dstFile.InsertFunc["p1"] = u.InsertFloatFromCursor(n.Body, "List", epsNodeIndex)
				nn := epsNode.Rhs[0].(*dst.CompositeLit)
				// 每次都在第一个位置插入
				dstFile.InsertFunc["p2"] = u.InsertFixed(nn, "Elts", 0)
			}

		}
	})
	fmt.Printf("allEndpoint.  MakeAllEndpoint函数>>>%s\n", a[hasFindMakeAll])
	fmt.Printf("allEndpoint.  AllEndpoint结构体>>>%s\n", a[hasFindAllEndpoint])
	fmt.Printf("allEndpoint.  AllEndpoint结构体>eps这个点>>>%s\n", a[hasFindEps])
	if !hasFindMakeAll || !hasFindAllEndpoint {
		return nil, nil, fmt.Errorf("MakeAllEndpoint函数,或者AllEndpoint结构体,或者eps点,没找到")
	}

	// 检查和插入的函数
	checkAndInsertFunc = func(paramsSlice []*templateParams) (err1 error) {
		for _, topicM := range paramsSlice {
			//===============================================================
			// 处理模板需要的参数,并插入
			for name, trans := range realParamsTrans {
				if vv, ok := topicM.p[trans.Need]; ok {
					if trans.F != nil {
						realParams[name] = trans.F(vv)
					} else {
						realParams[name] = vv
					}
					continue
				}
				err = fmt.Errorf("传入模板的参数错误")
				return
			}
			nodeMap, err2 := u.GetKeyNodeFromTemplateMap(t, realParams,
				filepath.Join(`./node_dst`, t, t+"_temp"+time.Now().Format("02150405")),
				nil, "k1", "k2", "k3", "k4")
			if err2 != nil {
				return err2
			}

			err1 = dstFile.InsertFromMapV2(nodeMap, &PositionAndKey{
				Position: "all",
				Keys:     []string{"k4"},
			}, &PositionAndKey{
				Position: "p1",
				Keys:     []string{"k1", "k2"},
			}, &PositionAndKey{
				Position: "p2",
				Keys:     []string{"k3"},
			})
			if err1 != nil {
				return err1
			}

		}
		return
	}
	err = checkAndInsertFunc(realSlice)
	return
}

//===============================================================

type InsertDstFileV2 struct {
	Tree       *dst.File
	FilePath   string
	InsertFunc map[string]func(n dst.Node) error // position->insertFunc
	src        *bytes.Buffer
}

// 确保m里的position存在
func (t *InsertDstFileV2) InitInsertFunc(f func(dst.Node, string, int, ...bool) func(n dst.Node) error, m map[string]*u.KeyNodePosition, positions ...string) {
	for _, p := range positions {
		node := m[p]
		t.InsertFunc[p] = f(node.Parent, node.FieldName, node.Index)
	}
}
func (t *InsertDstFileV2) Insert(position string, nodes ...dst.Node) error {
	if f, ok := t.InsertFunc[position]; ok {
		for _, node := range nodes {
			err := f(node)
			if err != nil {
				return err
			}
		}
	} else {
		panic(fmt.Errorf("Insert没找到对应的标识:%v\n", position))
	}
	return nil
}

type PositionAndKey struct {
	Position string
	Keys     []string
}

func NewInsertDstFileV2(node *dst.File, filePath string) *InsertDstFileV2 {
	return &InsertDstFileV2{
		Tree:       node,
		FilePath:   filePath,
		InsertFunc: make(map[string]func(n dst.Node) error),
	}
}

// m要保证一定有position
// 可以用模板生成,positions.Position用于找到对应的位置的插入函数,再从m获取node进行插入
func (t *InsertDstFileV2) InsertFromMap(m map[string]dst.Node, positions ...*PositionAndKey) error {
	for _, position := range positions {
		if insert, ok := t.InsertFunc[position.Position]; ok {
			for _, key := range position.Keys {
				node, o := m[key]
				if !o {
					panic(fmt.Errorf("没找到对应的key"))
				}
				err := insert(node)
				if err != nil {
					return err
				}
			}
			continue
		}
		panic(fmt.Errorf("没找到对应的插入位置标识"))
	}
	return nil
}
func (t *InsertDstFileV2) InsertFromMapV2(m map[string]*u.KeyNodePosition, positions ...*PositionAndKey) error {
	for _, position := range positions {
		if insert, ok := t.InsertFunc[position.Position]; ok {
			for _, key := range position.Keys {
				node, o := m[key]
				if !o {
					panic(fmt.Errorf("没找到对应的key"))
				}
				err := insert(node.Self)
				if err != nil {
					return err
				}
			}
			continue
		}
		panic(fmt.Errorf("没找到对应的插入位置标识"))
	}
	return nil
}
func NewInsertDstFileV2FromTemplateMap(templatePath string, param interface{}, filePath, wDir string, beforeKeys []string, afterKeys []string,
	f ...func(m map[string]*u.KeyNodePosition)) (*InsertDstFileV2, error) {
	//files, err := template.ParseFiles(templatePath)
	files, err := template.New(templatePath).Parse(temps.Temps[templatePath])
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer([]byte{})
	err = files.Execute(buffer, param)
	if err != nil {
		return nil, err
	}
	return NewInsertDstFileV2FromFile(filePath, buffer, wDir, beforeKeys, afterKeys, f...)
}

// 如果p开头的标识,不会删掉注释
// beforeKeys,afterKeys都是position
func NewInsertDstFileV2FromFile(filePath string, src interface{}, wDir string, beforeKeys []string, afterKeys []string,
	f ...func(m map[string]*u.KeyNodePosition)) (*InsertDstFileV2, error) {
	keys := append(beforeKeys[:], afterKeys[:]...)
	positionMap, err := u.GetKeyNodeFromFileV2(filePath, src, wDir, nil, keys...)
	if err != nil {
		return nil, err
	}
	if len(f) != 0 {
		f[0](positionMap)
	}
	newDst := NewInsertDstFileV2(positionMap[u.FileKey].Self.(*dst.File), filePath)
	newDst.InitInsertFunc(u.InsertFloat, positionMap, beforeKeys...)
	newDst.InitInsertFunc(u.InsertFloat, positionMap, afterKeys...)
	return newDst, err
}

func (t *InsertDstFileV2) CheckSrc() (err error) {
	if t.FilePath == "" || t.Tree == nil {
		return fmt.Errorf("filePath或者dstTree不能为空")
	}
	t.src = bytes.NewBuffer([]byte{})
	err = decorator.Fprint(t.src, t.Tree)
	return err
}

func (t *InsertDstFileV2) Src2File() (err error) {
	_, err = u.CreateDirOrFileAll(t.FilePath)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(t.FilePath, os.O_RDWR, 0777)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, t.src)
	return err
}
