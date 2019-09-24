package apis

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"gopkg.in/yaml.v2"
)

type Vmess struct {
	Add  string `json:"add"`
	Aid  int    `json:"aid"`
	Host string `json:"host"`
	ID   string `json:"id"`
	Net  string `json:"net"`
	Path string `json:"path"`
	Port string `json:"port"`
	PS   string `json:"ps"`
	TLS  string `json:"tls"`
	Type string `json:"type"`
	V    string `json:"v"`
}

type Clash struct {
	Port      int `yaml:"port"`
	SocksPort int `yaml:"socks-port"`
	// RedirPort          int                      `yaml:"redir-port"`
	// Authentication     []string                 `yaml:"authentication"`
	AllowLan           bool   `yaml:"allow-lan"`
	Mode               string `yaml:"mode"`
	LogLevel           string `yaml:"log-level"`
	ExternalController string `yaml:"external-controller"`
	// ExternalUI         string                   `yaml:"external-ui"`
	// Secret             string                   `yaml:"secret"`
	// Experimental       map[string]interface{} 	`yaml:"experimental"`
	Proxy             []map[string]interface{} `yaml:"Proxy"`
	ProxyGroup        []map[string]interface{} `yaml:"Proxy Group"`
	Rule              []string                 `yaml:"Rule"`
	CFWByPass         []string                 `yaml:"cfw-bypass"`
	CFWLatencyTimeout int                      `yaml:"cfw-latency-timeout"`
}

func (this *Clash) LoadTemplate(path string, vmesss []Vmess) []byte {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		log.Printf("[%s] template doesn't exist.", path)
		return nil
	}
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("[%s] template open the failure.", path)
		return nil
	}
	err = yaml.Unmarshal(buf, &this)
	if err != nil {
		log.Printf("[%s] Template format error.", path)
	}

	this.Proxy = nil

	var proxys []map[string]interface{}
	var proxies []string
	for _, c := range vmesss {
		proxy := make(map[string]interface{})
		proxy["name"] = c.PS
		proxy["type"] = "vmess"
		proxy["server"] = c.Add
		proxy["port"] = c.Port
		proxy["uuid"] = c.ID
		proxy["alterId"] = c.Aid
		proxy["cipher"] = "auto"
		if "" != c.TLS {
			proxy["tls"] = true
		} else {
			proxy["tls"] = false

		}
		proxys = append(proxys, proxy)
		this.Proxy = append(this.Proxy, proxy)
		proxies = append(proxies, c.PS)
	}

	this.Proxy = proxys

	for _, group := range this.ProxyGroup {
		groupProxies := group["proxies"].([]interface{})
		switch group["name"].(string) {
		case "ForeignMedia": // 国际媒体服务
			group["proxies"] = []string{"PROXY"}
		case "DomesticMedia": // 国内媒体服务
			group["proxies"] = []string{"DIRECT", "PROXY"}
		default:
			for i, proxie := range groupProxies {
				if "1" == proxie {
					groupProxies = groupProxies[:i]
					var tmpGroupProxies []string
					for _, s := range groupProxies {
						tmpGroupProxies = append(tmpGroupProxies, s.(string))
					}
					tmpGroupProxies = append(tmpGroupProxies, proxies...)
					group["proxies"] = tmpGroupProxies
					break
				}
			}
		}

	}

	d, err := yaml.Marshal(this)
	if err != nil {
		return nil
	}

	return d
}

func V2ray2Clash(c *gin.Context) {
	sublink := c.DefaultQuery("sub_link", "")

	if !strings.HasPrefix(sublink, "http") {
		c.String(http.StatusBadRequest, "参数错误.")
		return
	}
	resp, err := http.Get(sublink)

	if nil != err {
		c.String(http.StatusBadRequest, "sublink 不能访问")
		return
	}
	defer resp.Body.Close()
	s, err := ioutil.ReadAll(resp.Body)
	if nil != err || resp.StatusCode != http.StatusOK {
		c.String(http.StatusBadRequest, "sublink 不能访问")
		return
	}
	decodeBody, err := base64.RawStdEncoding.DecodeString(string(s))
	if nil != err || !strings.HasPrefix(string(decodeBody), "vmess://") {
		log.Println(err)
		c.String(http.StatusBadRequest, "sublink 返回数据格式不对")
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(decodeBody)))
	var vmesss []Vmess
	for scanner.Scan() {
		s := scanner.Text()[8:]
		s = strings.Trim(s, `\n`)
		vmconfig, err := base64.RawStdEncoding.DecodeString(s)
		if nil != err {
			continue
		}
		vmess := Vmess{}
		err = json.Unmarshal(vmconfig, &vmess)
		if err != nil {
			log.Fatalln(err)
			continue
		}
		vmesss = append(vmesss, vmess)
	}
	clash := Clash{}
	r := clash.LoadTemplate("ConnersHua.yaml", vmesss)
	if r == nil {
		c.String(http.StatusBadRequest, "sublink 返回数据格式不对")
	} else {
		c.String(http.StatusOK, string(r))
	}
	return

}
