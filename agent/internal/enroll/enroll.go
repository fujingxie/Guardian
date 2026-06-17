package enroll

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type Request struct {
	EnrollmentToken string `json:"enrollmentToken"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
}

type Response struct {
	ServerID   string `json:"serverId"`
	AgentToken string `json:"agentToken"`
}

// Do 用一次性 enrollment token 向控制台换长期 agent token。
func Do(consoleURL, enrollmentToken string, insecure bool) (*Response, error) {
	hn, _ := os.Hostname()
	req := Request{
		EnrollmentToken: enrollmentToken,
		Hostname:        hn,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
	}
	body, _ := json.Marshal(req)

	url := strings.TrimRight(consoleURL, "/") + "/api/agent/enroll"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, // 仅本地 localhost 自签时使用
		},
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("enroll http: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enroll failed (%d): %s", resp.StatusCode, string(data))
	}
	var out Response
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse enroll: %w (body=%s)", err, string(data))
	}
	if out.AgentToken == "" || out.ServerID == "" {
		return nil, fmt.Errorf("enroll response missing fields: %s", string(data))
	}
	return &out, nil
}
