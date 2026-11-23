package httpapi

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	geetestAPIURL      = "http://api.geetest.com"
	geetestRegisterURL = "/register.php"
	geetestValidateURL = "/validate.php"
	geetestJSONFormat  = "1"
	geetestNewCaptcha  = true
	geetestSDKVersion  = "mysecondwebgame-fiber:3.1.1"
)

const (
	geetestChallengeField = "geetest_challenge"
	geetestValidateField  = "geetest_validate"
	geetestSeccodeField   = "geetest_seccode"
)

type geetestLibResult struct {
	Status int
	Data   string
	Msg    string
}

func newGeetestLibResult(status int, data string, msg string) *geetestLibResult {
	return &geetestLibResult{Status: status, Data: data, Msg: msg}
}

type geetestLib struct {
	geetestID  string
	geetestKey string
	httpClient *http.Client
}

func newGeetestLib(geetestID string, geetestKey string) *geetestLib {
	return &geetestLib{
		geetestID:  geetestID,
		geetestKey: geetestKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (g *geetestLib) register(digestmod string, params map[string]string) *geetestLibResult {
	originChallenge := g.requestRegister(params)
	return g.buildRegisterResult(originChallenge, digestmod)
}

func (g *geetestLib) localRegister() *geetestLibResult {
	return g.buildRegisterResult("", "")
}

func (g *geetestLib) requestRegister(params map[string]string) string {
	outParams := make(map[string]string, len(params)+3)
	for key, value := range params {
		outParams[key] = value
	}
	outParams["gt"] = g.geetestID
	outParams["json_format"] = geetestJSONFormat
	outParams["sdk"] = geetestSDKVersion

	body, err := g.httpGet(geetestAPIURL+geetestRegisterURL, outParams)
	if err != nil {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	challenge, _ := payload["challenge"].(string)
	return challenge
}

func (g *geetestLib) buildRegisterResult(originChallenge string, digestmod string) *geetestLibResult {
	if originChallenge == "" || originChallenge == "0" {
		challenge := randomHexLower(32)
		data := map[string]any{
			"success":     0,
			"gt":          g.geetestID,
			"challenge":   challenge,
			"new_captcha": geetestNewCaptcha,
		}
		raw, _ := json.Marshal(data)
		return newGeetestLibResult(0, string(raw), "geetest bypass fail mode")
	}
	challenge := ""
	switch strings.ToLower(strings.TrimSpace(digestmod)) {
	case "sha256":
		challenge = g.sha256Encode(originChallenge + g.geetestKey)
	case "hmac-sha256":
		challenge = g.hmacSHA256Encode(originChallenge, g.geetestKey)
	default:
		challenge = g.md5Encode(originChallenge + g.geetestKey)
	}
	data := map[string]any{
		"success":     1,
		"gt":          g.geetestID,
		"challenge":   challenge,
		"new_captcha": geetestNewCaptcha,
	}
	raw, _ := json.Marshal(data)
	return newGeetestLibResult(1, string(raw), "")
}

func (g *geetestLib) successValidate(challenge string, validate string, seccode string, params map[string]string) *geetestLibResult {
	if !g.checkParam(challenge, validate, seccode) {
		return newGeetestLibResult(0, "", "challenge/validate/seccode required")
	}
	responseSeccode := g.requestValidate(challenge, validate, seccode, params)
	if responseSeccode == "" {
		return newGeetestLibResult(0, "", "request geetest validate failed")
	}
	if responseSeccode == "false" {
		return newGeetestLibResult(0, "", "geetest validate rejected")
	}
	return newGeetestLibResult(1, "", "")
}

func (g *geetestLib) failValidate(challenge string, validate string, seccode string) *geetestLibResult {
	if !g.checkParam(challenge, validate, seccode) {
		return newGeetestLibResult(0, "", "challenge/validate/seccode required in fail mode")
	}
	return newGeetestLibResult(1, "", "")
}

func (g *geetestLib) requestValidate(challenge string, validate string, seccode string, params map[string]string) string {
	outParams := make(map[string]string, len(params)+6)
	for key, value := range params {
		outParams[key] = value
	}
	outParams["seccode"] = seccode
	outParams["challenge"] = challenge
	outParams["validate"] = validate
	outParams["json_format"] = geetestJSONFormat
	outParams["captchaid"] = g.geetestID
	outParams["sdk"] = geetestSDKVersion

	body, err := g.httpPost(geetestAPIURL+geetestValidateURL, outParams)
	if err != nil {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	switch value := payload["seccode"].(type) {
	case string:
		return value
	case bool:
		if !value {
			return "false"
		}
	}
	return ""
}

func (g *geetestLib) checkParam(challenge string, validate string, seccode string) bool {
	return strings.TrimSpace(challenge) != "" && strings.TrimSpace(validate) != "" && strings.TrimSpace(seccode) != ""
}

func (g *geetestLib) httpGet(rawURL string, params map[string]string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}
	req.URL.RawQuery = query.Encode()
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("geetest request failed")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (g *geetestLib) httpPost(rawURL string, params map[string]string) (string, error) {
	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}
	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("geetest request failed")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (g *geetestLib) md5Encode(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (g *geetestLib) sha256Encode(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (g *geetestLib) hmacSHA256Encode(value string, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomHexLower(length int) string {
	if length <= 0 {
		return ""
	}
	buf := make([]byte, (length+1)/2)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", length)
	}
	return hex.EncodeToString(buf)[:length]
}
