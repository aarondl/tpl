package tpl

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/oxtoacart/bpool"
)

// Templates is a map of all parsed templates.
type Templates map[string]*template.Template

// Must helper for Load()
func Must(t Templates, err error) Templates {
	if err != nil {
		panic(err)
	}
	return t
}

var bufPool = bpool.NewBufferPool(10)

// Render the template into a writer. Uses a buffer pool to not be inefficient.
func (t Templates) Render(w http.ResponseWriter, name string, data interface{}) error {
	buf := bufPool.Get()
	defer bufPool.Put(buf)

	tmpl, ok := t[name]
	if !ok {
		return fmt.Errorf("Template named: %s does not exist", name)
	}

	err := tmpl.ExecuteTemplate(buf, "", data)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, buf)
	return err
}

// Load .tpl template files from dir, using layout as a base. Panics on
// failure to parse/load anything.
func Load(dir, partialDir, layout string, funcs template.FuncMap) (Templates, error) {
	tpls := make(Templates)

	b, err := ioutil.ReadFile(filepath.Join(dir, layout))
	if err != nil {
		return nil, fmt.Errorf("Could not load layout: %v", err)
	}

	layoutTpl, err := template.New("").Funcs(funcs).Parse(string(b))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse layout: %v", err)
	}

	err = filepath.Walk(partialDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if !strings.HasPrefix(filepath.Base(path), "_") {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("Could not create relative path: %v", err)
		}
		name := removeExtension(filepath.Base(rel))

		b, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to load partial (%s): %v", rel, err)
		}

		_, err = layoutTpl.New(name).Parse(string(b))
		if err != nil {
			return fmt.Errorf("Failed to parse partial: (%s): %v", rel, err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to load partials: %v", err)
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if path == filepath.Join(dir, layout) || strings.HasPrefix(filepath.Base(path), "_") {
			return nil
		}

		if err != nil {
			return fmt.Errorf("Could not walk directory: %v", err)
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("Could not create relative path: %v", err)
		}

		name := removeExtension(rel)

		b, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to load template (%s): %v", rel, err)
		}

		clone, err := layoutTpl.Clone()
		if err != nil {
			return fmt.Errorf("Failed to clone layout: %v", err)
		}
		t, err := clone.New("yield").Parse(string(b))
		if err != nil {
			return fmt.Errorf("Failed to parse template (%s): %v", rel, err)
		}

		tpls[name] = t
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to load templates: %v", err)
	}

	return tpls, nil
}

func removeExtension(path string) string {
	dot := strings.Index(path, ".")
	if dot >= 0 {
		return path[:dot]
	}
	return path
}
