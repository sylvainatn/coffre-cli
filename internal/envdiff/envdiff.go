// Package envdiff compare les variables d'un fichier local à celles de Coffre.
package envdiff

import "sort"

// Diff : ce qui sépare le fichier local de Coffre. Les listes sont triées.
type Diff struct {
	// Dans Coffre, absentes du fichier local.
	OnlyRemote []string
	// Dans le fichier local, absentes de Coffre.
	OnlyLocal []string
	// Présentes des deux côtés avec des valeurs différentes.
	Changed []string
}

func (d Diff) Empty() bool {
	return len(d.OnlyRemote) == 0 && len(d.OnlyLocal) == 0 && len(d.Changed) == 0
}

func Compare(local, remote map[string]string) Diff {
	var d Diff
	for k, rv := range remote {
		lv, ok := local[k]
		switch {
		case !ok:
			d.OnlyRemote = append(d.OnlyRemote, k)
		case lv != rv:
			d.Changed = append(d.Changed, k)
		}
	}
	for k := range local {
		if _, ok := remote[k]; !ok {
			d.OnlyLocal = append(d.OnlyLocal, k)
		}
	}
	sort.Strings(d.OnlyRemote)
	sort.Strings(d.OnlyLocal)
	sort.Strings(d.Changed)
	return d
}
