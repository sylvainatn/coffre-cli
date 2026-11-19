// Package dotenv lit et réécrit des fichiers .env en conservant tout ce que
// l'utilisateur y a mis : commentaires, lignes vides, ordre et style des valeurs.
package dotenv

import (
	"fmt"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// line est une ligne logique du fichier (une valeur entre guillemets peut en couvrir plusieurs).
type line struct {
	raw    string // texte d'origine, sans le retour à la ligne final
	key    string // vide pour un commentaire, une ligne vide ou une ligne illisible
	value  string
	export bool // `export KEY=...`
}

// File est un fichier .env analysé. Les lignes non reconnues sont conservées telles quelles.
type File struct {
	lines []line
}

// Parse analyse le contenu d'un fichier .env.
// Syntaxe acceptée : KEY=valeur, export KEY=valeur, guillemets doubles (avec \n \" \\),
// simples ou obliques (littéraux, éventuellement sur plusieurs lignes), commentaires # en fin de ligne.
func Parse(content string) (*File, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	raw := strings.Split(content, "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}

	f := &File{}
	for i := 0; i < len(raw); i++ {
		text := raw[i]
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			f.lines = append(f.lines, line{raw: text})
			continue
		}

		body := trimmed
		export := false
		if rest, ok := strings.CutPrefix(body, "export "); ok {
			body, export = strings.TrimSpace(rest), true
		}
		key, rest, ok := strings.Cut(body, "=")
		key = strings.TrimSpace(key)
		if !ok || !keyPattern.MatchString(key) {
			f.lines = append(f.lines, line{raw: text})
			continue
		}
		rest = strings.TrimLeft(rest, " \t")

		value, consumed, err := parseValue(rest, raw[i+1:])
		if err != nil {
			return nil, fmt.Errorf("ligne %d (%s) : %w", i+1, key, err)
		}
		f.lines = append(f.lines, line{
			raw:    strings.Join(raw[i:i+1+consumed], "\n"),
			key:    key,
			value:  value,
			export: export,
		})
		i += consumed
	}
	return f, nil
}

// parseValue lit la valeur qui suit `=`. Elle renvoie aussi le nombre de lignes
// supplémentaires utilisées (valeur entre guillemets sur plusieurs lignes).
func parseValue(rest string, next []string) (string, int, error) {
	if rest == "" {
		return "", 0, nil
	}
	quote := rest[0]
	if quote != '"' && quote != '\'' && quote != '`' {
		// Valeur nue : un # précédé d'un espace commence un commentaire
		if idx := strings.Index(rest, " #"); idx >= 0 {
			rest = rest[:idx]
		}
		if idx := strings.Index(rest, "\t#"); idx >= 0 {
			rest = rest[:idx]
		}
		return strings.TrimSpace(rest), 0, nil
	}

	text := rest[1:]
	for consumed := 0; ; consumed++ {
		if end := closingQuote(text, quote); end >= 0 {
			inner := text[:end]
			if quote == '"' {
				inner = unescapeDouble(inner)
			}
			return inner, consumed, nil
		}
		if consumed >= len(next) {
			return "", 0, fmt.Errorf("guillemet %c non fermé", quote)
		}
		text += "\n" + next[consumed]
	}
}

// closingQuote renvoie l'index du guillemet fermant (non échappé pour les guillemets doubles), ou -1.
func closingQuote(s string, quote byte) int {
	for i := 0; i < len(s); i++ {
		if quote == '"' && s[i] == '\\' {
			i++
			continue
		}
		if s[i] == quote {
			return i
		}
	}
	return -1
}

func unescapeDouble(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '"', '\\':
				b.WriteByte(s[i+1])
			default:
				// Échappement inconnu (\$…) : conservé tel quel, comme la plupart des outils
				b.WriteByte('\\')
				b.WriteByte(s[i+1])
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Values renvoie les variables du fichier. En cas de doublon, la dernière définition l'emporte.
func (f *File) Values() map[string]string {
	out := make(map[string]string)
	for _, l := range f.lines {
		if l.key != "" {
			out[l.key] = l.value
		}
	}
	return out
}

// Keys renvoie les noms des variables dans l'ordre du fichier, sans doublon.
func (f *File) Keys() []string {
	seen := make(map[string]bool)
	var keys []string
	for _, l := range f.lines {
		if l.key != "" && !seen[l.key] {
			seen[l.key] = true
			keys = append(keys, l.key)
		}
	}
	return keys
}

// Set remplace la valeur d'une variable existante (sa dernière définition) ou l'ajoute à la fin.
func (f *File) Set(key, value string) {
	for i := len(f.lines) - 1; i >= 0; i-- {
		if f.lines[i].key == key {
			l := &f.lines[i]
			l.value = value
			l.raw = Line(key, value)
			if l.export {
				l.raw = "export " + l.raw
			}
			return
		}
	}
	f.lines = append(f.lines, line{raw: Line(key, value), key: key, value: value})
}

// AppendComment ajoute une ligne de commentaire (précédée d'une ligne vide si le fichier n'est pas vide).
func (f *File) AppendComment(text string) {
	if len(f.lines) > 0 && strings.TrimSpace(f.lines[len(f.lines)-1].raw) != "" {
		f.lines = append(f.lines, line{})
	}
	f.lines = append(f.lines, line{raw: "# " + text})
}

// String renvoie le contenu du fichier, terminé par un retour à la ligne.
func (f *File) String() string {
	if len(f.lines) == 0 {
		return ""
	}
	parts := make([]string, len(f.lines))
	for i, l := range f.lines {
		parts[i] = l.raw
	}
	return strings.Join(parts, "\n") + "\n"
}

var barePattern = regexp.MustCompile(`^[A-Za-z0-9_./:@+,=%-]+$`)

// Line formate `KEY=valeur` de façon relisible par les outils courants
// (dotenv, python-dotenv, Docker Compose, Laravel, Symfony) :
// valeur nue si elle est simple, sinon entre guillemets simples (littéraux, sans interpolation de $),
// sinon entre guillemets doubles avec échappements (valeur multiligne ou contenant ').
func Line(key, value string) string {
	switch {
	case value == "" || barePattern.MatchString(value):
		return key + "=" + value
	case !strings.ContainsAny(value, "'\n\r"):
		return key + "='" + value + "'"
	default:
		r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
		return key + `="` + r.Replace(value) + `"`
	}
}
