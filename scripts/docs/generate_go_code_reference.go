package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type declBlock struct {
	Title string
	Start int
	End   int
	Names []string
}

func main() {
	rootFlag := flag.String("root", "", "repo root (default: current working directory)")
	outFlag := flag.String("out", "docs/docs/reference/code", "output directory (relative to repo root unless absolute)")
	includeTests := flag.Bool("include-tests", false, "include *_test.go files")
	overwrite := flag.Bool("overwrite", false, "overwrite existing generated docs")
	flag.Parse()

	repoRoot := *rootFlag
	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fatalf("get cwd: %v", err)
		}
		repoRoot = cwd
	}
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		fatalf("abs root: %v", err)
	}

	outDir := *outFlag
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(repoRoot, outDir)
	}

	dirs := flag.Args()
	if len(dirs) == 0 {
		dirs = []string{"cmd", "ingestor", "decoder", "sinks", "api", "bridge", "backfill"}
	}

	var goFiles []string
	for _, dir := range dirs {
		base := filepath.Join(repoRoot, dir)
		if _, err := os.Stat(base); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fatalf("stat %s: %v", base, err)
		}
		if err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			if !*includeTests && strings.HasSuffix(path, "_test.go") {
				return nil
			}
			goFiles = append(goFiles, path)
			return nil
		}); err != nil {
			fatalf("walk %s: %v", base, err)
		}
	}

	sort.Strings(goFiles)
	updated := 0
	skipped := 0

	for _, srcPath := range goFiles {
		rel, err := filepath.Rel(repoRoot, srcPath)
		if err != nil {
			fatalf("rel path: %v", err)
		}

		outPath := filepath.Join(outDir, strings.TrimSuffix(rel, ".go")+".mdx")
		if !*overwrite {
			if _, err := os.Stat(outPath); err == nil {
				skipped++
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			fatalf("mkdir %s: %v", filepath.Dir(outPath), err)
		}

		content, err := renderGoFileDoc(srcPath, filepath.ToSlash(rel))
		if err != nil {
			fatalf("render %s: %v", rel, err)
		}
		if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
			fatalf("write %s: %v", outPath, err)
		}
		updated++
	}

	fmt.Printf("generated=%d skipped=%d out=%s\n", updated, skipped, outDir)
}

func renderGoFileDoc(absPath string, relPathSlash string) (string, error) {
	src, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, src, parser.ParseComments)
	if err != nil {
		return "", err
	}

	blocks := buildDeclBlocks(fset, file)

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", relPathSlash))
	b.WriteString("---\n\n")

	b.WriteString("This page is an **annotated source reference**. Code blocks are imported from the repository at build time.\n\n")
	b.WriteString("## What this file is\n\n")
	b.WriteString("_TODO: one-paragraph summary of responsibility and boundaries._\n\n")
	b.WriteString("## Why this file exists\n\n")
	b.WriteString("_TODO: architectural motivation and why this logic lives here (vs elsewhere)._ \n\n")

	b.WriteString("## Full source\n\n")
	b.WriteString("```go ")
	b.WriteString(fmt.Sprintf("title=%q ", relPathSlash))
	b.WriteString(fmt.Sprintf("file=<rootDir>/%s ", relPathSlash))
	b.WriteString("showLineNumbers\n")
	b.WriteString("```\n\n")

	b.WriteString("## Walkthrough (by declaration)\n\n")
	for _, block := range blocks {
		b.WriteString(fmt.Sprintf("### %s\n\n", block.Title))
		if len(block.Names) > 0 {
			b.WriteString("**Symbols:** ")
			b.WriteString(strings.Join(block.Names, ", "))
			b.WriteString("\n\n")
		}

		b.WriteString("```go ")
		b.WriteString(fmt.Sprintf("title=%q ", block.Title))
		b.WriteString(fmt.Sprintf("file=<rootDir>/%s#L%d-L%d ", relPathSlash, block.Start, block.End))
		b.WriteString("showLineNumbers\n")
		b.WriteString("```\n\n")

		b.WriteString("**What:** _TODO_\n\n")
		b.WriteString("**How:** _TODO_\n\n")
		b.WriteString("**Why:** _TODO_\n\n")
	}

	return b.String(), nil
}

func buildDeclBlocks(fset *token.FileSet, file *ast.File) []declBlock {
	blocks := []declBlock{
		{
			Title: fmt.Sprintf("package %s", file.Name.Name),
			Start: fset.Position(file.Package).Line,
			End:   fset.Position(file.Name.End()).Line,
		},
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			blocks = append(blocks, declBlock{
				Title: formatFuncDeclTitle(fset, d),
				Start: startLineWithDoc(fset, d.Doc, d.Pos()),
				End:   fset.Position(d.End()).Line,
				Names: []string{d.Name.Name},
			})
		case *ast.GenDecl:
			names := genDeclNames(d)
			title := strings.ToLower(d.Tok.String())
			if title == "import" {
				title = "imports"
			}
			if len(names) > 0 {
				title = fmt.Sprintf("%s (%s)", title, strings.Join(names, ", "))
			}
			blocks = append(blocks, declBlock{
				Title: strings.TrimSpace(title),
				Start: startLineWithDoc(fset, d.Doc, d.Pos()),
				End:   fset.Position(d.End()).Line,
				Names: names,
			})
		}
	}

	// Keep stable ordering in the file.
	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].Start == blocks[j].Start {
			return blocks[i].End < blocks[j].End
		}
		return blocks[i].Start < blocks[j].Start
	})

	return blocks
}

func startLineWithDoc(fset *token.FileSet, doc *ast.CommentGroup, fallback token.Pos) int {
	if doc == nil {
		return fset.Position(fallback).Line
	}
	return fset.Position(doc.Pos()).Line
}

func genDeclNames(decl *ast.GenDecl) []string {
	unique := map[string]struct{}{}
	var names []string

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name != nil {
				name := s.Name.Name
				if _, ok := unique[name]; !ok {
					unique[name] = struct{}{}
					names = append(names, name)
				}
			}
		case *ast.ValueSpec:
			for _, n := range s.Names {
				name := n.Name
				if _, ok := unique[name]; !ok {
					unique[name] = struct{}{}
					names = append(names, name)
				}
			}
		}
	}

	sort.Strings(names)
	return names
}

func formatFuncDeclTitle(fset *token.FileSet, decl *ast.FuncDecl) string {
	recv := ""
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		recvType := formatNode(fset, decl.Recv.List[0].Type)
		recv = fmt.Sprintf("(%s) ", recvType)
	}
	// Keep titles MDX-safe: do not include full signatures because channel types
	// like "<-chan" can be mis-parsed as JSX.
	return fmt.Sprintf("func %s%s", recv, decl.Name.Name)
}

func formatNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	_ = format.Node(&buf, fset, node)
	return buf.String()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
