package main

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed index.html
var files embed.FS

type page struct {
	Root    string
	Backend string
	Message string
	Error   bool
}

func validBackend(raw string) (*url.URL, error) {
	address, err := url.Parse(raw)
	if err != nil || address.Scheme != "https" || address.Hostname() == "" || address.User != nil || address.RawQuery != "" || address.ForceQuery || address.Fragment != "" {
		return nil, fmt.Errorf("укажите HTTPS-адрес LChatHost без логина, параметров или фрагмента")
	}
	return address, nil
}

func probe(address *url.URL, secret string, roots *x509.CertPool) (bool, error) {
	target := *address
	target.Path = strings.TrimRight(target.Path, "/") + "/install.php"
	target.RawPath = ""
	request, err := http.NewRequest("GET", target.String(), nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("X-LChat-Gateway", "1")
	request.Header.Set("X-LChat-Proxy-Key", secret)
	request.Header.Set("X-LChat-Client-IP", "127.0.0.1")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return false, fmt.Errorf("LChatHost вернул HTTP %d", response.StatusCode)
	}
	var status struct {
		Done *bool `json:"done"`
	}
	if err = json.NewDecoder(http.MaxBytesReader(nil, response.Body, 8192)).Decode(&status); err != nil || status.Done == nil {
		return false, fmt.Errorf("адрес не вернул ожидаемый ответ LChatHost")
	}
	return *status.Done, nil
}

func phpString(value string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(value) + "'"
}

func saveConfig(root, address, secret string, replace bool) error {
	path := filepath.Join(root, "config.php")
	if _, err := os.Lstat(path); err == nil && !replace {
		return fmt.Errorf("config.php уже существует. Подтвердите его замену в форме")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	file, err := os.CreateTemp(root, ".lchat-web-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	config := "<?php\ndeclare(strict_types=1);\nif (!defined('LCHAT_INTERNAL')) { http_response_code(404); exit; }\nreturn ['backend' => ['mode' => 'pc', 'url' => " + phpString(address) + ", 'proxy_secret' => " + phpString(secret) + "]];\n"
	_, err = file.WriteString(config)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if !replace {
		if err = os.Link(file.Name(), path); err != nil {
			return fmt.Errorf("не удалось создать config.php: %w", err)
		}
		return nil
	}
	return os.Rename(file.Name(), path)
}

func main() {
	folder := flag.String("root", ".", "LChatWeb directory")
	noBrowser := flag.Bool("no-browser", false, "Print the local panel URL")
	version := flag.Bool("version", false, "Print version")
	caFile := flag.String("ca-file", "", "Optional PEM CA certificate for a private backend")
	flag.Parse()
	var roots *x509.CertPool
	if *caFile != "" {
		var err error
		roots, err = x509.SystemCertPool()
		if err != nil {
			log.Fatal(err)
		}
		pem, err := os.ReadFile(*caFile)
		if err != nil {
			log.Fatal(err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			log.Fatal("Invalid PEM certificate")
		}
	}
	if *version {
		fmt.Println("LChatWeb Panel 1.0.0")
		return
	}
	root, err := filepath.Abs(*folder)
	if err != nil {
		log.Fatal(err)
	}
	for _, name := range []string{"index.php", "gateway.php", "config.example.php"} {
		if fi, e := os.Stat(filepath.Join(root, name)); e != nil || fi.IsDir() {
			log.Fatal("Укажите папку LChatWeb через --root")
		}
	}
	panel := template.Must(template.ParseFS(files, "index.html"))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	host := listener.Addr().String()
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		log.Fatal(err)
	}
	path := "/" + hex.EncodeToString(token) + "/"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host || r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		state := page{Root: root}
		if r.Method == "POST" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != "null" && origin != "http://"+host {
				http.Error(w, "Forbidden", 403)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 8192)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Invalid form", 400)
				return
			}
			state.Backend = strings.TrimSpace(r.PostForm.Get("backend"))
			secret := r.PostForm.Get("secret")
			address, err := validBackend(state.Backend)
			if err == nil && (len(secret) < 32 || len(secret) > 512 || strings.ContainsAny(secret, " \r\n\t\x00") || strings.HasPrefix(secret, "REPLACE_")) {
				err = fmt.Errorf("введите общий секрет шлюза длиной от 32 символов без пробелов")
			}
			if err == nil {
				var done bool
				done, err = probe(address, secret, roots)
				if err == nil {
					state.Message = "Связь с LChatHost работает. Теперь откройте install.php на вашем сайте."
					if done {
						state.Message = "Связь с LChatHost работает. Таблицы уже созданы."
					}
					if r.PostForm.Get("action") == "save" {
						err = saveConfig(root, address.String(), secret, r.PostForm.Get("replace") == "yes")
						if err == nil {
							state.Message = "config.php сохранён. " + state.Message
						}
					}
				}
			}
			if err != nil {
				state.Message = err.Error()
				state.Error = true
			}
		} else if r.Method != "GET" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if err := panel.Execute(w, state); err != nil {
			log.Print(err)
		}
	})
	address := "http://" + host + path
	fmt.Println(address)
	if !*noBrowser {
		if runtime.GOOS == "windows" {
			_ = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", address).Start()
		} else {
			_ = exec.Command("xdg-open", address).Start()
		}
	}
	log.Fatal((&http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 25 * time.Second}).Serve(listener))
}
