package matcher

import (
	"encoding/base64"
	"io/ioutil"
	"regexp"
	"strings"
)

var (
	_ DomainMatcher = &ABPlus{}
)

// ABPlus 基于部分AdBlock Plus规则的域名匹配器
type ABPlus struct {
	isBlocked     map[string]bool
	blockedRegs   []*regexp.Regexp
	unblockedRegs []*regexp.Regexp
}

// Match 判断域名是否匹配ADBlock Plus规则
func (matcher *ABPlus) Match(domain string) (matched bool, ok bool) {
	if domain == "" {
		return
	}
	domain = strings.ToLower(domain)
	if domain[len(domain)-1] == '.' {
		domain = domain[:len(domain)-1] // 移除域名末尾的根域名
	}
	// 依次拆解域名进行匹配
	for suffix := domain; strings.Contains(suffix, "."); {
		if matched, ok = matcher.isBlocked[suffix]; ok {
			return // 对应记录则返回结果
		}
		if suffix[0] == '.' {
			suffix = suffix[1:] // 移除域名前的点号再匹配
		} else {
			suffix = suffix[strings.Index(suffix, "."):] // 移除最低级的域名再匹配
		}
	}
	// 通配符匹配
	for _, regex := range matcher.blockedRegs {
		if regex.MatchString(domain) {
			return true, true
		}
	}
	for _, regex := range matcher.unblockedRegs {
		if regex.MatchString(domain) {
			return false, true
		}
	}
	// 匹配失败
	return false, false
}

// Extend 将目标ABPlus对象规则添加到自身，规则重复时覆盖
func (matcher *ABPlus) Extend(target *ABPlus) {
	if target != nil {
		for domain, flag := range target.isBlocked {
			matcher.isBlocked[domain] = flag
		}
		matcher.blockedRegs = append(matcher.blockedRegs, target.blockedRegs...)
		matcher.unblockedRegs = append(matcher.unblockedRegs, target.unblockedRegs...)
	}
}

// NewABPByText 从文本内容读取AdBlock Plus规则
func NewABPByText(text string) (matcher *ABPlus) {
	extractDomain := func(rule string) string {
		// 从ABP规则中提取域名
		if i := strings.Index(rule, "||"); i != -1 {
			rule = rule[i+2:] // remove domain name anchor
		}
		if i := strings.Index(rule, "|"); i != -1 {
			rule = rule[i+1:] // remove address start anchor
		}
		if i := strings.Index(rule, "://"); i != -1 {
			rule = rule[i+3:] // remove method name
		}
		if i := strings.Index(rule, "/"); i != -1 {
			rule = rule[:i] // remove path of url
		}
		if i := strings.Index(rule, "^"); i != -1 {
			rule = rule[:i]
		}
		return rule
	}
	matcher = &ABPlus{isBlocked: map[string]bool{}}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '!' || line[0] == '[' {
			continue // 忽略空行、注释行、类型声明
		} else if line[0] == '/' { // path类规则
			if len(line) > 13 && line[:13] == "/^https?:\\/\\/" && line[len(line)-5:] == "\\/.*/" { // google正则补丁
				reg := regexp.MustCompile(line[13 : len(line)-5])
				matcher.blockedRegs = append(matcher.blockedRegs, reg)
			}
			continue
		}
		line = strings.Replace(line, "%2F", "/", -1)

		domain := extractDomain(line) // 提取规则中的域名
		// 判断域名中是否有通配符
		if strings.Index(domain, "*") != -1 {
			// 通配符表达式转正则表达式
			regStr := strings.Replace(domain, ".", "\\.", -1)
			regStr = strings.Replace(regStr, "*", ".*", -1)
			regex := regexp.MustCompile("^" + regStr + "$")
			if len(line) > 2 && line[:2] == "@@" {
				matcher.unblockedRegs = append(matcher.unblockedRegs, regex)
			} else {
				matcher.blockedRegs = append(matcher.blockedRegs, regex)
			}
			continue
		}
		// 通过顶级域名判断域名是否有效
		var tld string
		if i := strings.LastIndex(domain, "."); i == -1 {
			continue // 无顶级域名
		} else {
			tld = domain[i+1:]
		}
		tldReg := regexp.MustCompile(`^[a-zA-Z]{2,}$`)
		idnReg := regexp.MustCompile(`^xn--[a-zA-Z0-9]{3,}$`)
		if !tldReg.MatchString(tld) && !idnReg.MatchString(tld) {
			continue // 无效域名
		}
		domain = strings.ToLower(domain)
		matcher.isBlocked[domain] = line[:2] != "@@"
	}
	return matcher
}

// NewABPByFile 从文件内容读取AdBlock Plus规则
func NewABPByFile(filename string, b64decode bool) (checker *ABPlus, err error) {
	if filename == "" {
		return NewABPByText(""), nil
	}
	var raw []byte
	var text string
	if raw, err = ioutil.ReadFile(filename); err == nil {
		text = string(raw)
		if b64decode {
			if raw, err = base64.StdEncoding.DecodeString(text); err == nil {
				text = string(raw)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return NewABPByText(text), nil
}
