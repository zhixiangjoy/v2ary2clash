package api

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"gopkg.in/yaml.v2"
)

type Vmess struct {
	Add  string      `json:"add"`
	Aid  int         `json:"aid"`
	Host string      `json:"host"`
	ID   string      `json:"id"`
	Net  string      `json:"net"`
	Path string      `json:"path"`
	Port interface{} `json:"port"`
	PS   string      `json:"ps"`
	TLS  string      `json:"tls"`
	Type string      `json:"type"`
	V    string      `json:"v"`
}

type ClashVmess struct {
	Name           string      `json:"name,omitempty"`
	Type           string      `json:"type,omitempty"`
	Server         string      `json:"server,omitempty"`
	Port           interface{} `json:"port,omitempty"`
	UUID           string      `json:"uuid,omitempty"`
	AlterID        int         `json:"alterId,omitempty"`
	Cipher         string      `json:"cipher,omitempty"`
	TLS            bool        `json:"tls,omitempty"`
	Network        string      `json:"network,omitempty"`
	WSPATH         string      `json:"ws-path,omitempty"`
	WSHeaders      interface{} `json:"-"`
	SkipCertVerify bool        `json:"-"`
}

type ClashRSSR struct {
	Name          string      `json:"name,omitempty"`
	Type          string      `json:"type,omitempty"`
	Server        string      `json:"server,omitempty"`
	Port          interface{} `json:"port,omitempty"`
	Password      string      `json:"password,omitempty"`
	Cipher        string      `json:"cipher,omitempty"`
	Protocol      string      `json:"protocol,omitempty"`
	ProtocolParam string      `json:"protocolparam,omitempty"`
	OBFS          string      `json:"obfs,omitempty"`
	OBFSParam     string      `json:"obfsparam,omitempty"`
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

func (this *Clash) LoadTemplate(path string, protos []interface{}) []byte {
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

	switch protos[0].(type) {
	case ClashRSSR:
		for _, proto := range protos {
			c := proto.(ClashRSSR)
			proxy := make(map[string]interface{})
			j, _ := json.Marshal(proto)
			json.Unmarshal(j, &proxy)
			proxys = append(proxys, proxy)
			this.Proxy = append(this.Proxy, proxy)
			proxies = append(proxies, c.Name)
		}
		break
	case ClashVmess:
		for _, proto := range protos {
			c := proto.(ClashVmess)
			proxy := make(map[string]interface{})
			j, _ := json.Marshal(proto)
			json.Unmarshal(j, &proxy)
			proxys = append(proxys, proxy)
			this.Proxy = append(this.Proxy, proxy)
			proxies = append(proxies, c.Name)
		}
		break
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
func Base64DecodeStripped(s string) ([]byte, error) {
	if i := len(s) % 4; i != 0 {
		s += strings.Repeat("=", 4-i)
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(s)
	}
	return decoded, err
}

func V2ray2Clash(c *gin.Context) {
	rawURI := c.Request.URL.RawQuery
	if !strings.HasPrefix(rawURI, "sub_link=http") {
		c.String(http.StatusBadRequest, "sub_link=需要V2ray的订阅链接.")
		return
	}
	sublink := rawURI[9:]
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
	decodeBody, err := Base64DecodeStripped(string(s))
	if nil != err || !strings.HasPrefix(string(decodeBody), "vmess://") {
		log.Println(err)
		c.String(http.StatusBadRequest, "sublink 返回数据格式不对")
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(decodeBody)))
	var vmesss []interface{}
	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), "vmess://") {
			continue
		}
		s := scanner.Text()[8:]
		s = strings.Trim(s, `\n`)
		s = strings.Trim(s, `\r`)
		vmconfig, err := Base64DecodeStripped(s)
		if err != nil {
			continue
		}
		vmess := Vmess{}
		err = json.Unmarshal(vmconfig, &vmess)
		if err != nil {
			log.Fatalln(err)
			continue
		}
		clashVmess := ClashVmess{}
		clashVmess.Name = vmess.PS
		clashVmess.Type = "vmess"
		clashVmess.Server = vmess.Add
		switch vmess.Port.(type) {
		case string:
			clashVmess.Port, _ = vmess.Port.(string)
		case int:
			clashVmess.Port, _ = vmess.Port.(int)
		case float64:
			clashVmess.Port, _ = vmess.Port.(float64)
		default:
			continue
		}
		clashVmess.UUID = vmess.ID
		clashVmess.AlterID = vmess.Aid
		clashVmess.Cipher = "auto"
		if "" != vmess.TLS {
			clashVmess.TLS = true
		} else {
			clashVmess.TLS = false
		}
		if "ws" == vmess.Net {
			clashVmess.Network = vmess.Net
			clashVmess.WSPATH = vmess.Path
		}

		vmesss = append(vmesss, clashVmess)
	}
	clash := Clash{}
	r := clash.LoadTemplate("ConnersHua.yaml", vmesss)
	if r == nil {
		c.String(http.StatusBadRequest, "sublink 返回数据格式不对")
		return
	}
	c.String(http.StatusOK, string(r))
}

const (
	SSRServer = iota
	SSRPort
	SSRProtocol
	SSRCipher
	SSROBFS
	SSRSuffix
)

func SSR2ClashR(c *gin.Context) {
	rawURI := c.Request.URL.RawQuery
	if !strings.HasPrefix(rawURI, "sub_link=http") {
		c.String(http.StatusBadRequest, "sub_link=需要SSR的订阅链接.")
		return
	}
	sublink := rawURI[9:]
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
	decodeBody, err := Base64DecodeStripped(string(s))
	if nil != err || !strings.HasPrefix(string(decodeBody), "ssr://") {
		log.Println(err)
		c.String(http.StatusBadRequest, "sublink 返回数据格式不对")
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(decodeBody)))
	var ssrs []interface{}
	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), "ssr://") {
			continue
		}
		s := scanner.Text()[6:]
		s = strings.Trim(s, `\n`)
		s = strings.Trim(s, `\r`)
		rawSSRConfig, err := Base64DecodeStripped(s)
		if err != nil {
			continue
		}
		params := strings.Split(string(rawSSRConfig), `:`)
		if 6 != len(params) {
			continue
		}
		ssr := ClashRSSR{}
		ssr.Type = "ssr"
		ssr.Server = params[SSRServer]
		ssr.Port = params[SSRPort]
		ssr.Protocol = params[SSRProtocol]
		ssr.Cipher = params[SSRCipher]
		ssr.OBFS = params[SSROBFS]

		suffix := strings.Split(params[SSRSuffix], "/?")
		if 2 != len(suffix) {
			continue
		}
		passwordBase64 := suffix[0]
		password, err := Base64DecodeStripped(passwordBase64)
		if err != nil {
			continue
		}
		ssr.Password = string(password)

		m, err := url.ParseQuery(suffix[1])
		if err != nil {
			continue
		}
		for k, v := range m {
			de, err := Base64DecodeStripped(v[0])
			if err != nil {
				continue
			}
			switch k {
			case "obfsparam":
				ssr.OBFSParam = string(de)
				continue
			case "protoparam":
				ssr.ProtocolParam = string(de)
				continue
			case "remarks":
				ssr.Name = string(de)
				continue
			case "group":
				continue
			}
		}

		ssrs = append(ssrs, ssr)
	}
	clash := Clash{}
	r := clash.LoadTemplate("ConnersHua.yaml", ssrs)
	if r == nil {
		c.String(http.StatusBadRequest, "sublink 返回数据格式不对")
		return
	}
	c.String(http.StatusOK, string(r))
}
