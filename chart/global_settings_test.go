package chart

import (
	"bytes"
	"io"
	"os/exec"
	"testing"

	"gopkg.in/yaml.v3"
)

func render(t *testing.T, values string) []map[string]interface{} {
	t.Helper()
	cmd := exec.Command("helm", "template", "test", ".", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(values)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm template: %v\n%s", err, out)
	}
	dec := yaml.NewDecoder(bytes.NewReader(out))
	var docs []map[string]interface{}
	for {
		var doc map[string]interface{}
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
	return docs
}
func obj(m map[string]interface{}, key string) map[string]interface{} {
	return m[key].(map[string]interface{})
}
func deployment(t *testing.T, docs []map[string]interface{}) map[string]interface{} {
	t.Helper()
	for _, d := range docs {
		if d["kind"] == "Deployment" {
			return d
		}
	}
	t.Fatal("deployment missing")
	return nil
}
func TestGlobalSettings(t *testing.T) {
	global := `global:
  priorityClassName: global-priority
  nodeSelector: {pool: global}
  tolerations: [{key: global}]
  labels: {shared: global, app.kubernetes.io/name: wrong}
  podLabels: {pod: global, app.kubernetes.io/instance: wrong}
  image: {pullPolicy: Always}
`
	baseline := render(t, "")
	ignored := render(t, global)
	// The default path must produce the same manifests despite existing global keys.
	if !bytes.Equal(mustYAML(t, baseline), mustYAML(t, ignored)) {
		t.Fatal("global values changed output without opt-in")
	}
	docs := render(t, global+`useGlobalSettings: true
priorityClassName: local-priority
nodeSelector: {pool: local}
tolerations: [{key: local}]
commonLabels: {shared: local}
podLabels: {pod: local}
podAnnotations: {example.com/pod: value}
`)
	d := deployment(t, docs)
	template := obj(obj(d, "spec"), "template")
	meta := obj(template, "metadata")
	spec := obj(template, "spec")
	labels := obj(meta, "labels")
	if spec["priorityClassName"] != "local-priority" || obj(spec, "nodeSelector")["pool"] != "local" {
		t.Fatal("local scheduling values did not win")
	}
	tol := spec["tolerations"].([]interface{})
	if len(tol) != 2 || tol[0].(map[string]interface{})["key"] != "global" || tol[1].(map[string]interface{})["key"] != "local" {
		t.Fatal("unexpected toleration order")
	}
	if labels["shared"] != "local" || labels["pod"] != "local" || labels["app.kubernetes.io/name"] != "replicated" || labels["app.kubernetes.io/instance"] != "test" {
		t.Fatalf("unexpected labels: %v", labels)
	}
	if obj(meta, "annotations")["example.com/pod"] != "value" {
		t.Fatal("pod annotation missing")
	}
	for _, doc := range docs {
		if annotations, ok := obj(doc, "metadata")["annotations"].(map[string]interface{}); ok && annotations["example.com/pod"] != nil {
			t.Fatalf("pod annotation leaked to %v", doc["kind"])
		}
	}
	containers := spec["containers"].([]interface{})
	if containers[0].(map[string]interface{})["imagePullPolicy"] != "Always" {
		t.Fatal("global pull policy missing")
	}
	fallback := deployment(t, render(t, global+"useGlobalSettings: true\n"))
	fs := obj(obj(obj(fallback, "spec"), "template"), "spec")
	if fs["priorityClassName"] != "global-priority" || obj(fs, "nodeSelector")["pool"] != "global" {
		t.Fatal("global scheduling fallback missing")
	}
	override := deployment(t, render(t, global+"useGlobalSettings: true\nimage: {pullPolicy: Never}\n"))
	os := obj(obj(obj(override, "spec"), "template"), "spec")
	if os["containers"].([]interface{})[0].(map[string]interface{})["imagePullPolicy"] != "Never" {
		t.Fatal("local pull policy did not win")
	}
}
func mustYAML(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
