package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

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

type MapItem struct {
	Key, Value interface{}
}

type ClashConfig struct {
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
	Proxy      []map[string]interface{} `yaml:"Proxy"`
	ProxyGroup []map[string]interface{} `yaml:"Proxy Group"`
	Rule       []string                 `yaml:"Rule"`
}

func BuildClashConfig(vmesss []Vmess, rules []byte) []byte {
	clashConfig := ClashConfig{
		Port:               7890,
		SocksPort:          7891,
		AllowLan:           false,
		Mode:               "Rule",
		LogLevel:           "info",
		ExternalController: "0.0.0.0:9090",
	}
	// proxys := make([]map[string]interface{}, 0)
	autoGroup := make(map[string]interface{})
	autoGroup["name"] = "Auto"
	autoGroup["type"] = "select"
	autoGroup["proxies"] = []string{}
	autoGroup["url"] = "http://www.gstatic.com/generate_204"
	autoGroup["interval"] = 300

	proxyGroup := make(map[string]interface{})
	proxyGroup["name"] = "Proxy"
	proxyGroup["type"] = "select"
	proxyGroup["proxies"] = []string{"Auto"}

	// 国内模式
	domesticGroup := make(map[string]interface{})
	domesticGroup["name"] = "Domestic"
	domesticGroup["type"] = "select"
	domesticGroup["proxies"] = []string{"DIRECT", "Proxy"}
	// 中国
	chinaMediaGroup := make(map[string]interface{})
	chinaMediaGroup["name"] = "China_media"
	chinaMediaGroup["type"] = "select"
	chinaMediaGroup["proxies"] = []string{"Domestic", "Proxy"}
	// 全局
	globalMediaGroup := make(map[string]interface{})
	globalMediaGroup["name"] = "Global_media"
	globalMediaGroup["type"] = "select"
	globalMediaGroup["proxies"] = []string{"Proxy"}
	// 其它
	othersGroup := make(map[string]interface{})
	othersGroup["name"] = "Others"
	othersGroup["type"] = "select"
	othersGroup["proxies"] = []string{"Proxy", "Domestic"}

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
		clashConfig.Proxy = append(clashConfig.Proxy, proxy)
		proxyGroup["proxies"] = append(proxyGroup["proxies"].([]string), c.PS)
		autoGroup["proxies"] = append(autoGroup["proxies"].([]string), c.PS)
	}
	clashConfig.ProxyGroup = append(clashConfig.ProxyGroup, autoGroup)
	clashConfig.ProxyGroup = append(clashConfig.ProxyGroup, proxyGroup)
	clashConfig.ProxyGroup = append(clashConfig.ProxyGroup, domesticGroup)
	clashConfig.ProxyGroup = append(clashConfig.ProxyGroup, chinaMediaGroup)
	clashConfig.ProxyGroup = append(clashConfig.ProxyGroup, globalMediaGroup)
	clashConfig.ProxyGroup = append(clashConfig.ProxyGroup, othersGroup)

	err := yaml.Unmarshal(rules, &clashConfig)
	if err != nil {
		clashConfig.Rule = append(clashConfig.Rule, "MATCH,Proxy")
	}

	d, err := yaml.Marshal(&clashConfig)

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
	rulesbuf, err := ioutil.ReadFile("rules.yaml")
	if err != nil {
		fmt.Print(err)
	}
	r := BuildClashConfig(vmesss, rulesbuf)
	c.String(http.StatusOK, string(r))
	return
}

func main() {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/v2ray2clash", V2ray2Clash)

	srv := &http.Server{
		Addr:    "0.0.0.0:5050",
		Handler: router,
	}

	go func() {
		// 服务连接
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器（设置 5 秒的超时时间）
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	log.Println("Server exiting")
}
