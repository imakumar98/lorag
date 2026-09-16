package rag

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingEmbedder struct {
	calls     [][]string
	failTimes int
}

func (r *recordingEmbedder) EmbedDocuments(texts []string) ([][]float32, error) {
	copied := append([]string(nil), texts...)
	r.calls = append(r.calls, copied)
	if r.failTimes > 0 {
		r.failTimes--
		return nil, errors.New("tokenize EOF")
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = []float32{float32(len(text))}
	}
	return out, nil
}

func (r *recordingEmbedder) EmbedQuery(text string) ([]float32, error) {
	return []float32{float32(len(text))}, nil
}

func TestEmbedDocumentsSplitsIntoBatches(t *testing.T) {
	inner := &recordingEmbedder{}
	embeddings := BatchedEmbeddings{Inner: inner, BatchSize: 2}

	vectors, err := embeddings.EmbedDocuments([]string{"a", "bb", "ccc", "dddd", "e"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.calls) != 3 {
		t.Fatalf("calls = %#v", inner.calls)
	}
	if strings.Join(inner.calls[0], ",") != "a,bb" ||
		strings.Join(inner.calls[1], ",") != "ccc,dddd" ||
		strings.Join(inner.calls[2], ",") != "e" {
		t.Fatalf("calls = %#v", inner.calls)
	}
	if len(vectors) != 5 || vectors[0][0] != 1 || vectors[3][0] != 4 {
		t.Fatalf("vectors = %#v", vectors)
	}
}

func TestEmbedDocumentsRetriesAFailedBatch(t *testing.T) {
	inner := &recordingEmbedder{failTimes: 1}
	embeddings := BatchedEmbeddings{Inner: inner, BatchSize: 2}

	vectors, err := embeddings.EmbedDocuments([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.calls) != 2 {
		t.Fatalf("calls = %#v", inner.calls)
	}
	if strings.Join(inner.calls[0], ",") != "a,b" || strings.Join(inner.calls[1], ",") != "a,b" {
		t.Fatalf("calls = %#v", inner.calls)
	}
	if len(vectors) != 2 {
		t.Fatalf("vectors = %#v", vectors)
	}
}

func TestAnswerQuestionRequiresExistingIndex(t *testing.T) {
	dir := t.TempDir()
	_, _, err := AnswerQuestion("What is ACATS?", filepath.Join(dir, "docs"), filepath.Join(dir, "db"), "nomic-embed-text", "llama3.2:3b", AskDeps{})
	if err == nil || err.Error() != "No index found. Run `lorag sync`." {
		t.Fatalf("error = %v", err)
	}
	var qerr *QuestionError
	if !errors.As(err, &qerr) {
		t.Fatalf("error type = %T", err)
	}
}

func TestAnswerQuestionReturnsAnswerAndUniqueSources(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.Mkdir(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docsDir := filepath.Join(dir, "docs")

	var searchedQuery string
	var searchedK int
	var openedDocs, openedDB, openedEmbed string
	var chatModel, system, user string

	answer, sources, err := AnswerQuestion("What is ACATS?", docsDir, dbDir, "nomic-embed-text", "llama3.2:3b", AskDeps{
		OpenStore: func(docs, db, embed string) (VectorStore, error) {
			openedDocs, openedDB, openedEmbed = docs, db, embed
			return VectorStoreFunc(func(query string, k int) ([]Document, error) {
				searchedQuery, searchedK = query, k
				return []Document{
					{Source: "/tmp/a.txt", Content: "waived"},
					{Source: "/tmp/a.txt", Content: "also waived"},
					{Source: "/tmp/b.txt", Content: "other"},
				}, nil
			}), nil
		},
		Chat: func(model, sys, usr string) (string, error) {
			chatModel, system, user = model, sys, usr
			return "Fee is waived", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "Fee is waived" {
		t.Fatalf("answer = %q", answer)
	}
	if len(sources) != 2 || sources[0] != "/tmp/a.txt" || sources[1] != "/tmp/b.txt" {
		t.Fatalf("sources = %#v", sources)
	}
	if openedDocs != docsDir || openedDB != dbDir || openedEmbed != "nomic-embed-text" {
		t.Fatalf("open args %q %q %q", openedDocs, openedDB, openedEmbed)
	}
	if searchedQuery != "What is ACATS?" || searchedK != RetrievalK {
		t.Fatalf("search query=%q k=%d", searchedQuery, searchedK)
	}
	if chatModel != "llama3.2:3b" {
		t.Fatalf("chat model = %q", chatModel)
	}
	if !strings.Contains(system, "waived") || !strings.Contains(system, "/tmp/a.txt") {
		t.Fatalf("system = %q", system)
	}
	if user != "What is ACATS?" {
		t.Fatalf("user = %q", user)
	}
}

func TestAnswerQuestionMapsConnectionRefusedToOllamaError(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.Mkdir(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := AnswerQuestion("What is ACATS?", filepath.Join(dir, "docs"), dbDir, "nomic-embed-text", "llama3.2:3b", AskDeps{
		OpenStore: func(string, string, string) (VectorStore, error) {
			return nil, errors.New("connection refused")
		},
	})
	if err == nil || err.Error() != "Run lorag setup." {
		t.Fatalf("error = %v", err)
	}
}

func TestAnswerQuestionMapsModelNotFoundToMissingChatModel(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.Mkdir(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := AnswerQuestion("What is ACATS?", filepath.Join(dir, "docs"), dbDir, "nomic-embed-text", "llama3.2:3b", AskDeps{
		OpenStore: func(string, string, string) (VectorStore, error) {
			return VectorStoreFunc(func(string, int) ([]Document, error) {
				return nil, nil
			}), nil
		},
		Chat: func(string, string, string) (string, error) {
			return "", errors.New("model 'llama3.2:3b' not found")
		},
	})
	if err == nil || err.Error() != "Chat model is missing. Run `lorag model use <name>`." {
		t.Fatalf("error = %v", err)
	}
}

func TestAnswerQuestionDoesNotRemapUnrelatedModelErrors(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.Mkdir(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := AnswerQuestion("What is ACATS?", filepath.Join(dir, "docs"), dbDir, "nomic-embed-text", "llama3.2:3b", AskDeps{
		OpenStore: func(string, string, string) (VectorStore, error) {
			return nil, errors.New("embedding model failed")
		},
	})
	if err == nil || err.Error() != "embedding model failed" {
		t.Fatalf("error = %v", err)
	}
	var qerr *QuestionError
	if errors.As(err, &qerr) {
		t.Fatal("should not be QuestionError")
	}
}

func TestLoadDocumentsReadsTextAndSkipsUnknownTypes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("hello md"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.bin"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "sub")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := LoadDocuments(dir)
	if err != nil {
		t.Fatal(err)
	}
	bySource := map[string]string{}
	for _, doc := range docs {
		bySource[doc.Source] = doc.Content
	}
	if bySource[filepath.Join(dir, "note.md")] != "hello md" {
		t.Fatalf("md = %#v", docs)
	}
	if bySource[filepath.Join(dir, "note.txt")] != "hello txt" {
		t.Fatalf("txt = %#v", docs)
	}
	if bySource[filepath.Join(nested, "nested.txt")] != "nested" {
		t.Fatalf("nested = %#v", docs)
	}
	if _, ok := bySource[filepath.Join(dir, "skip.bin")]; ok {
		t.Fatal("included unknown suffix")
	}
}

type hashEmbedder struct{}

func (hashEmbedder) EmbedDocuments(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = hashVec(text)
	}
	return out, nil
}

func (hashEmbedder) EmbedQuery(text string) ([]float32, error) {
	return hashVec(text), nil
}

func hashVec(text string) []float32 {
	v := make([]float32, 8)
	for i, r := range text {
		v[i%8] += float32(int(r)%17 + 1)
	}
	return v
}

func TestRebuildCreatesIndexAndRetrievesSource(t *testing.T) {
	docsDir := t.TempDir()
	source := filepath.Join(docsDir, "fee.txt")
	if err := os.WriteFile(source, []byte("The ACATS fee is waived."), 0o644); err != nil {
		t.Fatal(err)
	}
	dbDir := filepath.Join(t.TempDir(), "database")
	embed := hashEmbedder{}
	if err := Rebuild(docsDir, dbDir, "dummy", embed); err != nil {
		t.Fatal(err)
	}

	store, err := openStore(dbDir, embed)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := store.SimilaritySearch("ACATS fee", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("docs = %#v", docs)
	}
	if docs[0].Source != source {
		t.Fatalf("source = %q", docs[0].Source)
	}
	if !strings.Contains(docs[0].Content, "waived") {
		t.Fatalf("content = %q", docs[0].Content)
	}
}
