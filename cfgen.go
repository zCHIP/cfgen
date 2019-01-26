package cfgen

import (
	"fmt"
	"io"
	"text/template"
)

const (
	dnsRoot string = "svc.cluster.local"

	templateContent string = `
location /{{.ServiceName}} {
    proxy_pass http://{{.ServiceFQDN}};
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Scheme $scheme;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Protocol $scheme;
    proxy_set_header X-Forwarded-Proto $scheme;
    # next 3 headers added to support websocket
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
`
)

type fields struct {
	ServiceName string
	ServiceFQDN string
}

func Generate(svcName, namespace string, wr io.Writer) error {
	// Creates the template
	t, err := template.New("NginxConfig").Parse(templateContent)
	if err != nil {
		return err
	}

	f := fields{svcName, fmt.Sprintf("%s.%s.%s", svcName, namespace, dnsRoot)}

	// Applying template
	if err := t.Execute(wr, f); err != nil {
		return err
	}

	return nil
}
