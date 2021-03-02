package utils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"genarate/temps"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unsafe"
)

type TagMap struct {
	m map[string]string
}

func NewTagMap() *TagMap {
	return &TagMap{
		m: make(map[string]string),
	}
}
func (t *TagMap) Set(k, v string) {
	t.m[k] = v
}
func (t *TagMap) Delete(k string) {
	delete(t.m, k)
}
func (t *TagMap) String() string {
	slice := make([]string, 0, len(t.m))
	for k := range t.m {
		slice = append(slice, k)
	}
	sort.Strings(slice)
	var b bytes.Buffer
	b.WriteString("`")
	for i, j := range slice {
		b.WriteString(j)
		b.WriteString(`:"`)
		b.WriteString(t.m[j])
		b.WriteString(`"`)
		if i < len(slice)-1 {
			b.WriteByte(' ')
		}
	}
	b.WriteByte('`')
	return b.String()
}
func ParseTag(srcTag string) (*TagMap, error) {
	s := strings.TrimFunc(srcTag, func(r rune) bool {
		if r == '`' {
			return true
		}
		return false
	})
	c := regexp.MustCompile(`.+?:".+?"`)
	allString := c.FindAllString(s, -1)
	tagMap := NewTagMap()
	for _, j := range allString {
		kv := strings.TrimSpace(j)
		kvSlice := strings.SplitN(kv, ":", 2)
		tagMap.Set(kvSlice[0], strings.Trim(kvSlice[1], "\""))
	}
	return tagMap, nil
}

// 将一个dstNode的语法树写到文件里, 注意:如果传入的文件名已经存在,会删掉之前的而重建一个
func WriteDst2File(x interface{}, fileName ...string) {
	var create *os.File
	var err error
	if len(fileName) == 0 {
		os.Remove("./dst.txt")
		create, err = os.Create("./dst.txt")
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		_, err = CreateDirOrFileAll(fileName[0])
		if err != nil {
			log.Fatalln(err)
		}
		create, err = os.OpenFile(fileName[0], os.O_RDWR, 0777)
		if err != nil {
			log.Fatalln(err)
		}
	}
	defer create.Close()
	buffer := bytes.NewBuffer([]byte{})
	err = dst.Fprint(buffer, x, func(name string, value reflect.Value) bool {
		return true
	})
	if err != nil {
		log.Fatalln(err)
	}
	scanner := bufio.NewScanner(buffer)
	b := bytes.NewBuffer([]byte{})
	scanner.Scan()
	s := scanner.Text()
	io.WriteString(create, strings.TrimPrefix(strings.TrimSpace(s), "0  ")+"\n")

	for scanner.Scan() {
		text := scanner.Text()
		b.Reset()
		flag := false
		for i, j := range text {
			if j == '.' && !flag {
				flag = true
			}
			if j == '\t' {
				b.WriteString("   ")
			}
			b.WriteByte(' ')
			if (j != '.' && j != ' ' && j != '\t') && flag {
				b.Write([]byte(text[i:]))
				break
			}
		}
		b.WriteByte('\n')
		b.Next(9)
		io.Copy(create, b)
	}
	//create.Seek(1, 2)
	io.WriteString(create, "}")
}

func Write2File2(fset *token.FileSet, x interface{}, fileName ...string) {
	var create *os.File
	if len(fileName) == 0 {
		os.Remove("./ast.txt")
		create, _ = os.Create("./ast.txt")
	} else {
		create, _ = os.Create(fileName[0])
	}

	buffer := bytes.NewBuffer([]byte{})
	ast.Fprint(buffer, fset, x, func(name string, value reflect.Value) bool {
		return true
	})

	scanner := bufio.NewScanner(buffer)
	b := bytes.NewBuffer([]byte{})
	scanner.Scan()
	s := scanner.Text()
	io.WriteString(create, strings.TrimPrefix(strings.TrimSpace(s), "0")+"\n")

	for scanner.Scan() {
		text := scanner.Text()
		b.Reset()
		flag := false
		for i, j := range text {
			if j == '.' && !flag {
				flag = true
			}
			b.WriteByte(' ')
			if (j != '.' && j != ' ' && j != '\t') && flag {
				b.Write([]byte(text[i:]))
				break
			}
		}
		b.WriteByte('\n')
		b.Next(9)
		io.Copy(create, b)
	}
	io.WriteString(create, "}")
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func FirstBigCase(name string) string {
	if name == "" {
		return ""
	}
	newstr := make([]byte, 1, len(name))
	if name[0] >= 'a' && name[0] <= 'z' {
		newstr[0] = name[0] - ('a' - 'A')
	} else {
		newstr[0] = name[0]
	}
	newstr = append(newstr, name[1:]...)
	return b2s(newstr)
}
func FirstSmallCase(name string) string {
	if name == "" {
		return ""
	}
	newstr := make([]byte, 1, len(name))
	if name[0] >= 'A' && name[0] <= 'Z' {
		newstr[0] = name[0] + ('a' - 'A')
	} else {
		newstr[0] = name[0]
	}
	newstr = append(newstr, name[1:]...)
	return b2s(newstr)
}

// 驼峰转蛇形
func Camel2SnackCase(name string) string {
	newstr := make([]byte, 0, len(name)+1)
	for i := 0; i < len(name); i++ {
		c := name[i]
		if isUpper := 'A' <= c && c <= 'Z'; isUpper {
			if i > 0 {
				newstr = append(newstr, '_')
			}
			c += 'a' - 'A'
		}
		newstr = append(newstr, c)
	}

	return b2s(newstr)
}

// 去除下划线,且将下划线后的字母大写, 默认将首字母小写 , if toBig[0]==true, 将首字母大写
func Snack2SmallCamelCase(name string) string {
	newstr := make([]byte, 0, len(name))
	upNextChar := false
	//if len(toBig)>0&&toBig[0] {
	//	upNextChar=true
	//}
	name = strings.ToLower(name)

	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case upNextChar:
			upNextChar = false
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
		case c == '_':
			upNextChar = true
			continue
		}

		newstr = append(newstr, c)
	}
	return b2s(newstr)
}

// 去除下划线,且将下划线后的字母大写, 将首字母大写
func Snack2BigCamelCase(name string) string {
	newstr := make([]byte, 0, len(name))
	name = strings.ToLower(name)
	upNextChar := true
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case upNextChar:
			upNextChar = false
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
		case c == '_':
			upNextChar = true
			continue
		}
		newstr = append(newstr, c)
	}
	return b2s(newstr)
}

func tempDir(m map[string]string) (dir string, err error) {
	if dir, err = ioutil.TempDir("", ""); err != nil {
		return
	}
	for fpathrel, src := range m {
		if strings.HasSuffix(fpathrel, "/") {
			// just a dir
			if err = os.MkdirAll(filepath.Join(dir, fpathrel), 0777); err != nil {
				return
			}
		} else {
			fpath := filepath.Join(dir, fpathrel)
			fdir, _ := filepath.Split(fpath)
			if err = os.MkdirAll(fdir, 0777); err != nil {
				return
			}

			var formatted []byte
			if strings.HasSuffix(fpath, ".go") {
				formatted, err = format.Source([]byte(src))
				if err != nil {
					err = fmt.Errorf("formatting %s: %v", fpathrel, err)
					return
				}
			} else {
				formatted = []byte(src)
			}

			if err = ioutil.WriteFile(fpath, formatted, 0666); err != nil {
				return
			}
		}
	}
	return
}

func Go2Txt(file string) error {
	open, err := os.Open(file)
	if err != nil {
		return err
	}
	defer open.Close()
	ext := path.Ext(file)
	suffix := strings.TrimSuffix("./template/func1.go", ext)
	create, err := os.Create(suffix + ".txt")
	if err != nil {
		return err
	}
	defer create.Close()
	_, err = io.Copy(create, open)
	if err != nil {
		return err
	}
	return nil
}

// TODO:  如果key相同,要报错,需要调用的人来做判断, 可以自定义一个 map, map.Add() ,map.Result()map[] ,最后返回一个map
// TODO:  还有,可以传入一个期待的 key的列表, 防止 key写错地方没找到
func GetNodeDst2File(node dst.Node, dir ...string) (key string, err error) {
	flag := "/*g:"
	suffix := "*/"
	dir0 := "./node_dst"
	if len(dir) != 0 {
		dir0 = dir[0]
	}
	_, err = CreateDirOrFileAll(dir0)
	if err != nil {
		return "", err
	}
	if node != nil {
		if a := node.Decorations(); a != nil {
			for i, j := range a.Start {
				if strings.HasPrefix(j, flag) {
					a.Start = append(a.Start[:i], a.Start[i+1:]...) //去掉这一行注释
					key = strings.TrimSuffix(j[len(flag):], suffix)
					WriteDst2File(node, path.Join(dir0, key+".txt"))
				}
			}
		}
	}
	return key, nil
}

func MakeDirAll(dir string) (isExist bool, err error) {
	exist, err := PathExists(dir)
	if err != nil {
		return false, err
	}
	if exist {
		return true, nil
	}
	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return false, err
	}
	return false, nil
}
func GetOneKey2File(d *decorator.Decorator, node dst.Node, srcFile io.ReadSeeker,
	commentDeleteFilter func(string) bool, dir ...string) (keys []string, err error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}
	keys, err = GetOneKeyDst2File(node, false, "", "", commentDeleteFilter, dir...)

	if len(keys) == 0 || err != nil {
		return keys, err
	}
	for _, key := range keys {
		err = GetOneKeyCode2File(d, node, srcFile, key, dir...)
		if err != nil {
			return keys, err
		}
	}
	return
}

func GetOneKeyCode2File(d *decorator.Decorator, node dst.Node, srcFile io.ReadSeeker, key string, dir ...string) error {
	start := d.Ast.Nodes[node].Pos()
	end := d.Ast.Nodes[node].End()
	_, err := srcFile.Seek(int64(start)-1, 0)
	if err != nil {
		return err
	}
	slice := make([]byte, end-start)
	_, err2 := io.ReadFull(srcFile, slice)
	if err2 != nil {
		return err2
	}
	dir0 := "./node_dst"
	if len(dir) != 0 {
		dir0 = dir[0]
	}
	f := filepath.Join(dir0, key+"_cod.txt")
	_, err = CreateDirOrFileAll(f)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(f, os.O_RDWR, 0777)
	if err != nil {
		return err
	}
	_, err = file.Write(slice)
	return err
}

// 打印传进来的node,以及其下的带有key的node
func GetKeyDst2FileFast(node dst.Node, dir ...string) (err error) {
	dir0 := "./node_dst"
	if len(dir) != 0 {
		dir0 = dir[0]
	}
	WriteDst2File(node, filepath.Join(dir0, "AllNode_dst.txt"))
	dstutil.Apply(node, func(cursor *dstutil.Cursor) bool {
		n := cursor.Node()
		_, err = GetOneKeyDst2File(n, false, "", "", func(s string) bool {
			return false
		}, dir...)
		if err != nil {
			panic(err)
		}
		return true
	}, nil)
	return err
}

// 判断节点是否有某个注释, 默认形如 `/*g:key1*/`, 就传入 startFlag=`/*g:`,suffixFlag=`*/` 目录默认是 ./node_dst
// 如果节点有,就会写入到文件中 默认  ./node_dst/key_dst.txt
// 返回找到的 key
// commentDeleteFilter: 入参为key,判断是否要删除标识key的注释, =nil则默认删除
func GetOneKeyDst2File(node dst.Node, onlyStart bool, startFlag string, suffixFlag string,
	commentDeleteFilter func(string) bool, dir ...string) (keys []string, err error) {
	flag := "/*g:"
	if startFlag != "" {
		flag = startFlag
	}
	suffix := "*/"
	if suffixFlag != "" {
		suffix = suffixFlag
	}
	dir0 := "./node_dst"
	if len(dir) != 0 {
		if filepath.Ext(dir[0]) != "" {
			return nil, fmt.Errorf("需要一个目录,而不是文件")
		}
		dir0 = dir[0]
	}
	_, err = CreateDirOrFileAll(dir0)
	if err != nil {
		return nil, err
	}
	if commentDeleteFilter == nil {
		commentDeleteFilter = func(s string) bool {
			return true
		}
	}
	if onlyStart { //如果只查 start位置的注释
		if a := node.Decorations(); a != nil {
			var currentRemove bool
			var nextRemove bool
			for i, j := range a.Start {
				currentRemove = nextRemove
				nextRemove = false
				hasFlag := strings.HasPrefix(j, flag)
				if !hasFlag {
					if currentRemove && j == "\n" {
						a.Start = append(a.Start[:i], a.Start[i+1:]...) //去掉这一行注释,这里直接操作的是原本的slice,也就是*node.xx
					}
				} else {
					key := strings.TrimSuffix(j[len(flag):], suffix)
					keys = append(keys, key)
					if commentDeleteFilter(key) {
						a.Start = append(a.Start[:i], a.Start[i+1:]...) //去掉这一行注释,这里直接操作的是原本的slice,也就是*node.xx
						if strings.HasPrefix(flag, "/*") {              //如果这一次有前缀,并且是/*开头的,要消除下一次的换行 /n
							nextRemove = true
						}
					}
					WriteDst2File(node, filepath.Join(dir0, key+"_dst.txt"))

				}

			}
		}
	} else {
		_, _, info := dstutil.Decorations(node)
		for _, j := range info { // j是一个节点的可能出现注释的位置的注释列表, 因为不同种类的节点可能有独有的注释位置
			var currentRemove bool
			var preKey string
			var nextRemove bool
			var deleteCount int // 通过反射改变切片长度,range的index还是原来的
			for i, k := range j.Decs {
				currentRemove = nextRemove
				nextRemove = false
				hasFlag := strings.HasPrefix(k, flag)
				if !hasFlag {
					if currentRemove && k == "\n" {
						nodeValue := reflect.ValueOf(node)
						fieldValue := reflect.Indirect(nodeValue).FieldByName("Decs")
						v := fieldValue.FieldByName(j.Name)
						v = reflect.Indirect(v)
						j.Decs = append(j.Decs[:i-deleteCount], j.Decs[i+1-deleteCount:]...) //去掉标志的注释, 但是是通过一个slice复制体,没有
						v.SetLen(v.Len() - 1)
						deleteCount++
						WriteDst2File(node, filepath.Join(dir0, preKey+"_dst.txt")) // 不然会有个多余的 \n ,覆写一下
					}
				} else {
					key := strings.TrimSuffix(k[len(flag):], suffix)
					keys = append(keys, key)
					//if key == "k2" {
					//	fmt.Println(key)
					//}
					if commentDeleteFilter(key) {
						nodeValue := reflect.ValueOf(node)
						fieldValue := reflect.Indirect(nodeValue).FieldByName("Decs")
						v := fieldValue.FieldByName(j.Name)
						v = reflect.Indirect(v)
						j.Decs = append(j.Decs[:i-deleteCount], j.Decs[i+1-deleteCount:]...) //去掉标志的注释, 但是是通过一个slice复制体,没有
						v.SetLen(v.Len() - 1)
						if strings.HasPrefix(flag, "/*") { //如果这一次有前缀,并且是/*开头的,要消除下一次的换行 /n
							nextRemove = true
						}
						deleteCount++
					}
					preKey = key
					WriteDst2File(node, filepath.Join(dir0, key+"_dst.txt"))
				}
				//因为j.Decs是 *node.Decs=[]string, 也就是一个slice的复制,

				// 比如: s1:=[]string{0,1,2,3,4} s2:=s1 s2=append(s2[:1],s2[2:]...)
				// 此时: s2=0,2,3,4       len(s2)
				//      s1=0,2,3,4,4
				// 所以用反射把 s1的length -1

			}
		}
		//for _, j := range info { // j是一个节点的可能出现注释的位置的注释列表, 因为不同种类的节点可能有独有的注释位置
		//	var currentRemove bool
		//	var nextRemove bool
		//	for i, k := range j.Decs {
		//		currentRemove = nextRemove
		//		nextRemove = false
		//		hasFlag := strings.HasPrefix(k, flag)
		//		if !hasFlag && !(currentRemove && k =="\n" ) {
		//			continue
		//		}
		//		if hasFlag {
		//			key := strings.TrimSuffix(k[len(flag):], suffix)
		//			keys = append(keys, key)
		//			WriteDst2File(node, filepath.Join(dir0, key+"_dst.txt"))
		//
		//			if key == "k2" {
		//				fmt.Println(key)
		//			}
		//			if !commentDeleteFilter(key) {
		//				continue
		//			}
		//			if strings.HasPrefix(flag, "/*") { //如果这一次有前缀,并且是/*开头的,要消除下一次的换行 /n
		//				nextRemove = true
		//			}
		//		}
		//
		//		nodeValue := reflect.ValueOf(node)
		//		fieldValue := reflect.Indirect(nodeValue).FieldByName("Decs")
		//		v := fieldValue.FieldByName(j.Name)
		//		v = reflect.Indirect(v)
		//		j.Decs = append(j.Decs[:i], j.Decs[i+1:]...) //去掉标志的注释, 但是是通过一个slice复制体,没有
		//		v.SetLen(v.Len() - 1)
		//		//因为j.Decs是 *node.Decs=[]string, 也就是一个slice的复制,
		//
		//		// 比如: s1:=[]string{0,1,2,3,4} s2:=s1 s2=append(s2[:1],s2[2:]...)
		//		// 此时: s2=0,2,3,4       len(s2)
		//		//      s1=0,2,3,4,4
		//		// 所以用反射把 s1的length -1
		//
		//	}
		//}
	}

	return keys, nil
}

// 创建文件或者目录,通过传入的参数的后缀来判断是文件or目录
// 如果文件已经存在,默认删除再新建, 目录已经存在会直接返回
// pathName:文件或者目录
// notRemove: true=如果文件已经存在就不进行任何操作
func CreateDirOrFileAll(pathName string, notRemove ...bool) (isExist bool, err error) {
	// 0.通过后缀判断文件or目录
	var isDir bool
	ext := filepath.Ext(pathName)
	if ext == "" {
		isDir = true
	}

	// 1.判断是否已经存在
	exist, err := PathExists(pathName)
	if err != nil {
		return false, err
	}
	if exist {
		if isDir { //如果是目录直接返回
			return true, nil
		}
		if len(notRemove) != 0 && notRemove[0] { //如果是文件, 且不进行remove,直接返回
			return true, nil
		} else {
			_ = os.Remove(pathName)
		}
	}
	// 1.创建目录
	var dir string
	if isDir {
		dir = pathName
	} else {
		dir = filepath.Dir(pathName) //如果传进来的是一个不带`/`结尾目录,这里可能就是实际目录的父目录了
	}
	err = os.MkdirAll(dir, os.ModePerm)
	if isDir || err != nil {
		return false, err
	}
	// 3.这里表示入参是一个文件
	create, err := os.Create(pathName)
	if err != nil {
		return false, err
	}
	err = create.Close()
	return false, err
}

// 获取不带后缀的文件名, 要确保是一个文件的路径,而不是目录
func GetFileNamePure(filePath string) string {
	ext := filepath.Ext(filePath)
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, ext)
}

// 判断文件或目录是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// 适用于一个目录只有一个package,一般就是这个情况了
func ParseDir(path string, filter func(os.FileInfo) bool) (p *dst.Package, d *decorator.Decorator, fset *token.FileSet, err error) {
	fset = token.NewFileSet()
	packages, err := parser.ParseDir(fset, path, filter, parser.ParseComments)
	if err != nil {
		return
	}
	d = decorator.NewDecorator(fset)
	for _, j := range packages {
		node, err := d.DecorateNode(j)
		if err != nil {
			return nil, nil, nil, err
		}
		p = node.(*dst.Package)
		return p, d, fset, nil
	}
	return nil, nil, nil, errors.New("dst miss packages")
}

func ParseFile(filename string, src interface{}) (f *dst.File, d *decorator.Decorator, fset *token.FileSet, err error) {
	fset = token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return
	}
	d = decorator.NewDecorator(fset)
	f, err = d.DecorateFile(file)
	return
}

const FileKey = "file"

// GetKeyNodeFromTemplate 从template中生成的代码遍历dst,获取带有 /*g:key1*/注释的节点,如果key重复有冲突,就会终止遍历且报错
// 同时,会在当前文件夹的node_dst目录生成对应的dst语法树和语法树所指的代码
// templatePath: 模板的路径
// param: 模板的参数
// dir: 节点语法树和对应代码保存的目录,没有会新建目录, 为空使用默认 ./node_dst目录
// expectKeys: 模板里节点的标志key,如果expectKeys的key搜寻不到,就报错
// m:返回一个 key:node 的map,也就是每个注释对应的dst节点,每个节点的key注释都是被删掉了的,但是显示的节点code保持原样,另有 FileKey:*dst.File
func GetKeyNodeFromTemplate(templatePath string, param interface{}, dir string, expectKeys ...string) (m map[string]dst.Node, err error) {
	files, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer([]byte{})
	err = files.Execute(buffer, param)
	//src := "package template\n" + buffer.String()
	src := buffer.String()
	f, d, _, err := ParseFile("", src)
	if err != nil {
		return nil, fmt.Errorf("ParseFile()错误:%v", err)
	}
	realDir := `./node_dst`
	if dir != "" {
		realDir = dir
	}
	m = make(map[string]dst.Node)
	var keys []string //为了错误可以输出,err那里就可以使用 = 而不是 :=
	reader := bytes.NewReader([]byte(src))
	dstutil.Apply(f, func(cursor *dstutil.Cursor) bool {
		return true
	}, func(cursor *dstutil.Cursor) bool {
		// 写在这个后置函数post,而不是pre中, 是因为想获取去除了/*g:key1*/注释的节点,而遍历树是深度优先,要先把小节点的注释去掉,大节点才不会有小节点的注释
		// 更正: 好像因为是指针,先啊后啊都没啥区别
		defer func() { keys = nil }()
		n := cursor.Node()
		if n == nil {
			return true
		}
		keys, err = GetOneKey2File(d, n, reader, nil, realDir)
		if err != nil {
			return false
		}
		for _, key := range keys {
			if _, ok := m[key]; ok {
				err = fmt.Errorf("关键字key有冲突:%v", key)
				return false
			}
			m[key] = n
		}
		return true
	})
	if err != nil {
		return m, err
	}
	for _, j := range expectKeys {
		if _, ok := m[j]; !ok {
			return m, fmt.Errorf("搜索到的key与期待不符,缺失:%v", j)
		}
	}
	m[FileKey] = f
	return m, err
}

type KeyNodePosition struct {
	Self      dst.Node
	Parent    dst.Node
	FieldName string
	Index     int
}

// commentDeleteFilter: 如果返回true,就删除标识key的注释, =nil,默认不删除`p`开头的注释
func GetKeyNodeFromFileV2(filePath string, src interface{}, wDir string, commentDeleteFilter func(string) bool, expectKeys ...string) (m map[string]*KeyNodePosition, err error) {
	var reader io.ReadSeeker
	file, d, _, err := ParseFile(filePath, src)
	if err != nil {
		return nil, err
	}
	realDir := `./node_dst/GetKeyNodeFromFileV2`
	if wDir != "" {
		realDir = wDir
	}
	if src != nil {
		switch s := src.(type) {
		case string:
			reader = strings.NewReader(s)
		case []byte:
			reader = bytes.NewReader(s)
		case *bytes.Buffer:
			reader = bytes.NewReader(s.Bytes())
		}
	} else {
		if filePath == "" {
			return nil, fmt.Errorf("GetKeyNodeFromFileV2()解析目标不能为空")
		}
		reader, err = os.Open(filePath)
		if err != nil {
			return nil, err
		}
	}
	if commentDeleteFilter == nil {
		commentDeleteFilter = func(s string) bool {
			return !strings.HasPrefix(s, "p")
		}
	}
	WriteDst2File(file, filepath.Join(realDir, "AllNode_dst.txt"))
	m = make(map[string]*KeyNodePosition)
	var keys []string
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		return true
	}, func(cursor *dstutil.Cursor) bool {
		// 写在这个后置函数post,而不是pre中, 是因为想获取去除了/*g:key1*/注释的节点,而遍历树是深度优先,要先把小节点的注释去掉,大节点才不会有小节点的注释
		defer func() { keys = nil }()
		n := cursor.Node()
		if n == nil {
			return true
		}
		keys, err = GetOneKey2File(d, n, reader, commentDeleteFilter, realDir)
		if err != nil {
			return false
		}
		for _, key := range keys {
			if _, ok := m[key]; ok {
				err = fmt.Errorf("关键字key有冲突\n.filePath:%v\n.key:%v\n", filePath, key)
				return false
			}
			m[key] = &KeyNodePosition{
				Self:      cursor.Node(),
				Parent:    cursor.Parent(),
				FieldName: cursor.Name(),
				Index:     cursor.Index(),
			}

		}
		return true
	})
	if err != nil {
		return m, err
	}
	for _, j := range expectKeys {
		if _, ok := m[j]; !ok {
			return m, fmt.Errorf("搜索到的key与期待不符,缺失:%v", j)
		}
	}
	m[FileKey] = &KeyNodePosition{
		Self: file,
	}
	return m, err
}

// GetKeyNodeFromTemplate 从template中生成的代码遍历dst,获取带有 /*g:key1*/注释的节点,如果key重复有冲突,就会终止遍历且报错
// 同时,会在当前文件夹的node_dst目录生成对应的dst语法树和语法树所指的代码
// templatePath: 模板的路径
// param: 模板的参数
// dir: 节点语法树和对应代码保存的目录,没有会新建目录, 为空使用默认 ./node_dst目录
// expectKeys: 模板里节点的标志key,如果expectKeys的key搜寻不到,就报错
// m:返回一个 key:node 的map,也就是每个注释对应的dst节点,每个节点的key注释都是被删掉了的,但是显示的节点code保持原样,另有 FileKey:*dst.File
func GetKeyNodeFromTemplateV2(templatePath string, param interface{}, wDir string,
	deleteComment func(string) bool, expectKeys ...string) (m map[string]*KeyNodePosition, err error) {
	files, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer([]byte{})
	err = files.Execute(buffer, param)
	if err != nil {
		return nil, err
	}
	return GetKeyNodeFromFileV2("", buffer, wDir, deleteComment, expectKeys...)
}
func GetKeyNodeFromTemplateMap(templatePath string, param interface{}, wDir string,
	deleteComment func(string) bool, expectKeys ...string) (m map[string]*KeyNodePosition, err error) {
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
	return GetKeyNodeFromFileV2("", buffer, wDir, deleteComment, expectKeys...)
}

//// 注意:如果在一个数组上面标记了两个点,也就是调用了连个insert函数,会导致 前一个点调用insert改变了数组长度,
//// 虽然我对单个insert函数做了处理,但是由于两个insert函数作用于同一个数组,后一个点的insert函数会受到影响,却没有做处理
//// 警告: 不要同时对一个数组使用 两个insert,不要一边遍历数组,且对当前数组进行insert,由于这是反射是通过新造一个切片,覆盖原来的切片
//// 而for-range在遍历之前会复制原来的切片(底层数组不复制),**不对:正因为反射覆盖了(也就是一个新数组),遍历又是对原来的数组,所以没问题
//// 但是还是不要在同一个数组上面用两个insert
//// TODO:  修改使用了insert函数的地方
//func InsertBefore(parent dst.Node, fieldName string, index int) func(n dst.Node) error {
//	if index < 0 {
//		panic("下标不能为负")
//	}
//	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
//	return func(n dst.Node) error {
//		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
//		l := v.Len()
//		reflect.Copy(v.Slice(index+1, l), v.Slice(index, l))
//		v.Index(index).Set(reflect.ValueOf(n))
//		index++
//		return nil
//	}
//}
//func InsertStart2(parent dst.Node, fieldName string, index int) func(n dst.Node) error {
//	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
//	return func(n dst.Node) error {
//		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
//		l := v.Len()
//		reflect.Copy(v.Slice(index+1, l), v.Slice(index, l))
//		v.Index(index).Set(reflect.ValueOf(n))
//		index++
//		return nil
//	}
//}
//
//// 顺序为 [node,3,2,1]  其中1,2,3的1是第一个插入的节点,3是最后调用函数插入的节点
//// 也就是 先入在底下
//func InsertAfter(parent dst.Node, fieldName string, index int) func(n dst.Node) error {
//	if index < 0 {
//		panic("下标不能为负")
//	}
//	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
//	return func(n dst.Node) error {
//		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
//		l := v.Len()
//		reflect.Copy(v.Slice(index+2, l), v.Slice(index+1, l))
//		v.Index(index + 1).Set(reflect.ValueOf(n))
//		//index++  //after不要增加下标
//		return nil
//	}
//}
//
//// 顺序为 [node,1,2,3]  其中1,2,3的1是第一个插入的节点,3是最后调用函数插入的节点
//// 也就是 后来的在底下
//func InsertEnd(parent dst.Node, fieldName string, index int) func(n dst.Node) error {
//	if index < 0 {
//		panic("下标不能为负")
//	}
//	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
//	return func(n dst.Node) error {
//		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
//		l := v.Len()
//		reflect.Copy(v.Slice(index+2, l), v.Slice(index+1, l))
//		v.Index(index + 1).Set(reflect.ValueOf(n))
//		index++
//		return nil
//	}
//}
//
//// 这个是针对于自己 index=len(slice)-1 可能会出现 index<0的情况
//// 如果是从Apply的cursor.Index()获取的,不应该用这个,用InsertEnd就行
//func InsertEnd2(parent dst.Node, fieldName string, index int) func(n dst.Node) error {
//	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
//	return func(n dst.Node) error {
//		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
//		l := v.Len()
//		reflect.Copy(v.Slice(index+2, l), v.Slice(index+1, l))
//		v.Index(index + 1).Set(reflect.ValueOf(n))
//		index++
//		return nil
//	}
//}

/*
	如果想不断在结尾位置插入,用 insertFloat(,,len(slice)-1,true) 或 insertFloat(,,len(slice))
	如果想不断在开始位置插入,用 insertFixed(,,0)
    //如果想不断在结尾位置插入,用 insertFloat(,,len(slice))
	//如果想不断在开始位置插入,用 insertFixed(,,0)
	//一般默认用 index作为参数,就是insertBefore, 加个1,表示insertAfter
	注意:对于从cursor获取的index,对于0这个入参,会有歧义:
	1.想insertAfter,通过index()返回-1,自己又加了个1,这个应该报错的;
	2.想在0位置insertBefore
	所以,在需要由工具类来判断,使用 后缀为 FromCursor的
	//而在传参使用lengthPoint而不是index,因为如果使用index,当传入len(slice)-1 ,

*/

//   | i | i |  -> len0 index0 len1 index1 len2
// 移动插入, length点不断加,会往下刷一排小怪,
// index=[-1,len-1] ,, lengthPoint=[0,len]
// 如果是在 Apply遍历的时候,不会出现数组长度等于0的情况,调用index(),要用这个 InsertFloatFromCursor
func InsertFloat(parent dst.Node, fieldName string, index int, isAfter ...bool) func(n dst.Node) error {
	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
	//虽然当 isAfter=false时,index=length 并不越界,其等效于 index=length-1,isAfter=true
	if index < -1 || index > v.Len()-1 {
		panic("insert时下标越界")
	}
	if len(isAfter) != 0 && isAfter[0] {
		index++
	}
	return func(n dst.Node) error {
		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
		l := v.Len()
		reflect.Copy(v.Slice(index+1, l), v.Slice(index, l))
		v.Index(index).Set(reflect.ValueOf(n))
		index++
		return nil
	}
}

//   | i | i |  -> len0 index0 len1 index1 len2
// 定点插入, 会在某个length点不断地刷出小怪, 使用这个,也就是要插入的点始终会在一个绝对位置出现,而不是相对于一个点(点是上图的`|`)
// index=[-1,len-1]
// 如果是在 Apply遍历的时候,调用index(),要用这个 InsertFixedFromCursor
func InsertFixed(parent dst.Node, fieldName string, index int, isAfter ...bool) func(n dst.Node) error {
	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
	//虽然当 isAfter=false时,index=length 并不越界,其等效于 index=length-1,isAfter=true
	if index < -1 {
		panic("insert时下标越界")
	}
	if len(isAfter) != 0 && isAfter[0] {
		index++
	}
	return func(n dst.Node) error {
		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
		l := v.Len()
		reflect.Copy(v.Slice(index+1, l), v.Slice(index, l))
		v.Index(index).Set(reflect.ValueOf(n))
		return nil
	}
}

// 如果是在 Apply遍历的时候,调用index(),要用这个
func InsertFloatFromCursor(parent dst.Node, fieldName string, index int, isAfter ...bool) func(n dst.Node) error {
	if index < 0 {
		panic("下标不能为负,说明这个field不是数组")
	}
	return InsertFloat(parent, fieldName, index, isAfter...)
}

// 如果是在 Apply遍历的时候,调用index(),要用这个
func InsertFixedFromCursor(parent dst.Node, fieldName string, index int, isAfter ...bool) func(n dst.Node) error {
	if index < 0 {
		panic("下标不能为负,说明这个field不是数组")
	}
	return InsertFixed(parent, fieldName, index, isAfter...)
}

type InsertFunc struct {
	count int
}

// TODO:
func (s *InsertFunc) InsertAfter(parent dst.Node, fieldName string, index int) func(n dst.Node) error {
	if index < 0 {
		panic("下标不能为负")
	}
	v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
	return func(n dst.Node) error {
		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
		l := v.Len()
		reflect.Copy(v.Slice(index+2, l), v.Slice(index+1, l))
		v.Index(index + 1).Set(reflect.ValueOf(n))
		//index++  //after不要增加下标
		return nil
	}
}

//===============================================================
// 控制深度的遍历

const (
	DeepDive     = 0 // 继续深度优先的遍历
	DeepContinue = 1 // 停止在当前深度
	DeepBreak    = 2 // 终止所有遍历
)

type Deep struct {
	deep int // 这个用来控制遍历的深度
	next int // 控制一下次是否 继续深入或者直接终止
	err  error
}

// 会直接终止 DeepApply 的遍历,使用 panic
func (d *Deep) Break(err error) {
	d.err = err
	d.next = DeepBreak
}
func (d *Deep) MovePan() {
	d.next = DeepContinue
}
func (d *Deep) Reset() {
	d.next = DeepDive
}
func (d *Deep) Deep() int {
	return d.deep
}
func (d *Deep) Err(err error) {
	d.err = err
}

var cBreak = new(int)
var cMovePan = new(int)

func DeepApply(node dst.Node, f func(cursor *dstutil.Cursor, c *Deep)) (err error) {
	c := new(Deep)
	defer func() {
		if r := recover(); r != nil && r != cBreak {
			panic(r)
		}
		err = c.err
	}()
	dstutil.Apply(node, func(cursor *dstutil.Cursor) (result bool) {
		defer func() {
			if result {
				c.deep++
			}
		}()
		f(cursor, c)
		if c.next == DeepBreak {
			panic(cBreak)
		}
		if c.next == DeepContinue {
			c.Reset()
			return false
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		c.deep--
		return true
	})
	return c.err
}

//===============================================================
