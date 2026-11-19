// Package envformat met les secrets au format attendu par les outils : .env, JSON, YAML,
// fichier d'environnement Docker, Secret Kubernetes.
package envformat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/coffre-dev/coffre-cli/internal/dotenv"
)

// Formats acceptés par --format.
var Formats = []string{"dotenv", "json", "yaml", "docker", "k8s"}

// Variable : un secret, dans l'ordre où l'API le renvoie (trié par nom).
type Variable struct {
	Key   string
	Value string
}

var k8sName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// Render produit le contenu dans le format demandé. `name` sert au Secret Kubernetes.
func Render(format string, vars []Variable, name string) (string, error) {
	var b strings.Builder
	switch format {
	case "dotenv", "":
		for _, v := range vars {
			b.WriteString(dotenv.Line(v.Key, v.Value) + "\n")
		}
	case "json":
		obj := make(map[string]string, len(vars))
		for _, v := range vars {
			obj[v.Key] = v.Value
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(obj); err != nil {
			return "", err
		}
		b.Write(buf.Bytes())
	case "yaml":
		for _, v := range vars {
			b.WriteString(v.Key + ": " + yamlString(v.Value) + "\n")
		}
	case "docker":
		// `docker run --env-file` : KEY=valeur brute, une ligne par variable, sans guillemets
		for _, v := range vars {
			if strings.ContainsAny(v.Value, "\n\r") {
				return "", fmt.Errorf("%s contient un retour à la ligne : impossible au format docker (utilisez dotenv)", v.Key)
			}
			b.WriteString(v.Key + "=" + v.Value + "\n")
		}
	case "k8s":
		if !k8sName.MatchString(name) || len(name) > 253 {
			return "", fmt.Errorf("nom de Secret Kubernetes invalide : %q (minuscules, chiffres et -)", name)
		}
		b.WriteString("apiVersion: v1\nkind: Secret\nmetadata:\n  name: " + name + "\ntype: Opaque\nstringData:\n")
		for _, v := range vars {
			b.WriteString("  " + v.Key + ": " + yamlString(v.Value) + "\n")
		}
	default:
		return "", fmt.Errorf("format inconnu %q (formats : %s)", format, strings.Join(Formats, ", "))
	}
	return b.String(), nil
}

// yamlString : une chaîne JSON est une chaîne YAML entre guillemets doubles valide.
func yamlString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(buf.String(), "\n")
}
