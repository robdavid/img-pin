package yaml_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/errors/test"
	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yamlv3 "gopkg.in/yaml.v3"
)

func TestPathString(t *testing.T) {
	p := yaml.PathOf("top", "middle", 1, 2, "bottom")
	assert.Equal(t, "top.middle[1][2].bottom", p.String())
}

func TestPathStringEscaping(t *testing.T) {
	p := yaml.PathOf("top", "middle.earth", 1, "[orcs]", 2, "bottom")
	assert.Equal(t, `top."middle.earth"[1]."[orcs]"[2].bottom`, p.String())
}

func readTestFile(file string) []byte {
	return Try(os.ReadFile(filepath.Join("tests", file)))
}

func TestMultiMatch(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	bytes := readTestFile("values.yaml")
	var node yamlv3.Node
	Check(yamlv3.Unmarshal(bytes, &node))
	repos := yaml.MatchManyWithNode(&node, yaml.PathOf("**", "image", "repository"))
	assert.Greater(len(repos), 0)
	assert.Equal(10, len(repos))
	for _, repo := range repos {
		suffix := repo.Path.Objects()[len(repo.Path)-2:]
		assert.Equal([]any{"image", "repository"}, suffix)
		assert.Equal("image", suffix[0])
		assert.Equal("repository", suffix[1])
		fmt.Println(repo.Path)
	}
}

func TestMultiMatchRepoAndTag(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	bytes := readTestFile("values.yaml")
	var node yamlv3.Node
	Check(yamlv3.Unmarshal(bytes, &node))
	repos := yaml.MatchManyWithNode(&node, yaml.PathOf("**", "image", "/^(repository|tag)$/"))
	assert.Greater(len(repos), 0)
	assert.Equal(20, len(repos))
	for _, repo := range repos {
		suffix := repo.Path.Objects()[len(repo.Path)-2:]
		assert.Equal("image", suffix[0])
		assert.True(suffix[1] == "repository" || suffix[1] == "tag")
		fmt.Println(repo.Path)
	}
}

func TestMultiMatchZeroMatch(t *testing.T) {
	const content = `
image:
  repository: ubuntu
my:
  image:
    repository: ubuntu
my:
  nested:
    image:
      repository: ubuntu
my:
  listed:
    - thing
    - other thing
    - image:
        repository: ubuntu
`
	defer test.ReportErr(t)
	assert := assert.New(t)
	bytes := []byte(content)
	var node yamlv3.Node
	Check(yamlv3.Unmarshal(bytes, &node))

	repos := yaml.MatchManyWithNode(&node, yaml.PathOf("**", "image", "repository"))
	assert.Equal(4, len(repos))
	assert.Equal([]any{"image", "repository"}, repos[0].Path.Objects())
	assert.Equal([]any{"my", "image", "repository"}, repos[1].Path.Objects())
	assert.Equal([]any{"my", "nested", "image", "repository"}, repos[2].Path.Objects())
	assert.Equal([]any{"my", "listed", 2, "image", "repository"}, repos[3].Path.Objects())

	repos = yaml.MatchManyWithNode(&node, yaml.PathOf("my", "**", "image", "repository"))
	assert.Equal(3, len(repos))
	assert.Equal([]any{"my", "image", "repository"}, repos[0].Path.Objects())
	assert.Equal([]any{"my", "nested", "image", "repository"}, repos[1].Path.Objects())
	assert.Equal([]any{"my", "listed", 2, "image", "repository"}, repos[2].Path.Objects())

	repos = yaml.MatchManyWithNode(&node, yaml.PathOf("my", "**", "**", "repository"))
	assert.Equal(3, len(repos))
	assert.Equal([]any{"my", "image", "repository"}, repos[0].Path.Objects())
	assert.Equal([]any{"my", "nested", "image", "repository"}, repos[1].Path.Objects())
	assert.Equal([]any{"my", "listed", 2, "image", "repository"}, repos[2].Path.Objects())

	repos = yaml.MatchManyWithNode(&node, yaml.PathOf("my", "listed", "**", "repository"))
	assert.Equal(1, len(repos))
	assert.Equal([]any{"my", "listed", 2, "image", "repository"}, repos[0].Path.Objects())

}

func TestMultiMatchRegex(t *testing.T) {
	const content = `
image:
  repository: ubuntu
  tag: 24.04
my:
  image:
    repository: ubuntu
my:
  nested:
    image:
      repository: ubuntu
my:
  listed:
    - thing
    - other thing
    - image:
        repository: ubuntu
registry:
  registry:
    image:
      repository: debian
      tag: bookworm
`
	defer test.ReportErr(t)
	assert := assert.New(t)
	bytes := []byte(content)
	var node yamlv3.Node
	Check(yamlv3.Unmarshal(bytes, &node))

	repos := yaml.MatchManyWithNode(&node, yaml.PathOf("**", "image", `/^(repository|tag)$/`))
	assert.Equal(7, len(repos))
	assert.Equal([]any{"image", "repository"}, repos[0].Path.Objects())
	assert.Equal([]any{"image", "tag"}, repos[1].Path.Objects())
	assert.Equal([]any{"my", "image", "repository"}, repos[2].Path.Objects())
	assert.Equal([]any{"my", "nested", "image", "repository"}, repos[3].Path.Objects())
	assert.Equal([]any{"my", "listed", 2, "image", "repository"}, repos[4].Path.Objects())
	assert.Equal([]any{"registry", "registry", "image", "repository"}, repos[5].Path.Objects())
	assert.Equal([]any{"registry", "registry", "image", "tag"}, repos[6].Path.Objects())
}

func TestGet(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	docs := Try(yaml.ReadDocs(filepath.Join("tests", "harbor.yaml")))
	kind := yaml.Get[string](docs[1], "kind")
	assert.Equal(opt.Value("HelmChart"), kind)
	repo := yaml.Get[string](docs[1], "spec", "repo")
	assert.Equal(opt.Value("https://helm.goharbor.io"), repo)
	nothing := yaml.Get[string](docs[1], "spec", "nothing")
	assert.True(nothing.IsEmpty())
}

func TestAliasedNodes(t *testing.T) {
	const content = `
default: &defaultFlower
  roses: red
garden:
  myflower: *defaultFlower
`
	defer test.ReportErr(t)
	assert := assert.New(t)
	require := require.New(t)
	bytes := []byte(content)
	var node yamlv3.Node
	Check(yamlv3.Unmarshal(bytes, &node))

	flowers := yaml.MatchManyWithNode(&node, yaml.PathOf("garden", "myflower", "roses"))
	require.Equal(1, len(flowers))
	assert.Equal([]any{"garden", "myflower", "roses"}, flowers[0].Path.Objects())
}

func TestGroupByPrefix(t *testing.T) {
	defer test.ReportErr(t)
	assert := assert.New(t)
	bytes := readTestFile("values.yaml")
	var node yamlv3.Node
	Check(yamlv3.Unmarshal(bytes, &node))
	entries := yaml.MatchManyWithNode(&node, yaml.PathOf("**", "image", "/^(repository|tag)$/"))
	assert.Equal(20, len(entries))

	buckets := yaml.GroupByPrefix(entries)

	// Each pair of (repository, tag) shares a prefix, so we expect 10 buckets
	assert.Equal(10, len(buckets))

	total := 0
	for _, b := range buckets {
		// Each bucket should have exactly 2 entries (repository and tag)
		assert.Equal(2, len(b.Entries))
		total += len(b.Entries)

		// Verify each entry's prefix matches the bucket prefix
		for _, e := range b.Entries {
			var expectedPrefix yaml.Path
			if len(e.Path) > 0 {
				expectedPrefix = e.Path[:len(e.Path)-1]
			}
			assert.True(expectedPrefix.Equals(b.Prefix),
				"entry path %v should have prefix %v, got %v", e.Path, b.Prefix, expectedPrefix)

			// Last element should be "repository" or "tag"
			lastObj := e.Path.Objects()[len(e.Path)-1]
			assert.Contains([]any{"repository", "tag"}, lastObj)
			// Decode the node value to confirm it's meaningful
			var val string
			assert.Nil(e.Node.Decode(&val))
			assert.NotEmpty(val)
		}
	}
	assert.Equal(20, total)
}

func TestPathSort(t *testing.T) {
	assert := assert.New(t)
	p1 := yaml.PathOf("a", "c", "d")
	p2 := yaml.PathOf("a", "b", "c")
	p3 := yaml.PathOf("a", "b", "c", 3)
	p4 := yaml.PathOf("a", "b", "c", 10)
	paths := []yaml.Path{p1, p2, p3, p4}
	yaml.SortPaths(paths)
	assert.Equal([]any{"a", "b", "c"}, paths[0].Objects())
	assert.Equal([]any{"a", "b", "c", 3}, paths[1].Objects())
	assert.Equal([]any{"a", "b", "c", 10}, paths[2].Objects())
	assert.Equal([]any{"a", "c", "d"}, paths[3].Objects())
}

func TestWriteNode(t *testing.T) {
	defer test.ReportErr(t)

	t.Run("new key in mapping", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("{}\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("a"), 1))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf("a"))
		assert.True(ok)
		var val int
		Check(result.Decode(&val))
		assert.Equal(1, val)
	})

	t.Run("update existing scalar", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("a: 1\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("a"), 2))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf("a"))
		assert.True(ok)
		var val int
		Check(result.Decode(&val))
		assert.Equal(2, val)
	})

	t.Run("new index in sequence", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("[10]\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf(1), 20))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf(0))
		assert.True(ok)
		var val int
		Check(result.Decode(&val))
		assert.Equal(10, val)

		result, ok = yaml.MatchOne(&doc, yaml.PathOf(1))
		assert.True(ok)
		Check(result.Decode(&val))
		assert.Equal(20, val)
	})

	t.Run("update existing index", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("[1, 2, 3]\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf(1), 5))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf(1))
		assert.True(ok)
		var val int
		Check(result.Decode(&val))
		assert.Equal(5, val)
	})

	t.Run("create parent mapping", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("{}\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("a", "b"), 1))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf("a", "b"))
		assert.True(ok)
		var val int
		Check(result.Decode(&val))
		assert.Equal(1, val)
	})

	t.Run("create parent sequence", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("{}\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("a", 0), "x"))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf("a", 0))
		assert.True(ok)
		var val string
		Check(result.Decode(&val))
		assert.Equal("x", val)
	})

	t.Run("deep nesting", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("{}\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("a", "b", "c"), 42))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf("a", "b", "c"))
		assert.True(ok)
		var val int
		Check(result.Decode(&val))
		assert.Equal(42, val)

		// Verify the tree structure directly
		mapping := doc.Content[0]
		assert.Equal(yamlv3.MappingNode, mapping.Kind)
		assert.Equal("a", mapping.Content[0].Value)
		assert.Equal(yamlv3.MappingNode, mapping.Content[1].Kind)
		nested := mapping.Content[1]
		assert.Equal("b", nested.Content[0].Value)
		assert.Equal(yamlv3.MappingNode, nested.Content[1].Kind)
	})

	t.Run("collision leaf is mapping", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("a:\n  b: 1\n"), &doc))
		err := yaml.WriteNode(&doc, yaml.PathOf("a"), 2)
		assert.Error(err)
		assert.ErrorIs(err, yaml.ErrWriteCollision)
	})

	t.Run("collision intermediate is scalar", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("a: 1\n"), &doc))
		err := yaml.WriteNode(&doc, yaml.PathOf("a", "b"), 2)
		assert.Error(err)
		assert.ErrorIs(err, yaml.ErrWriteCollision)
	})

	t.Run("collision wrong container type", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("a:\n  - 1\n"), &doc))
		// "a" holds a sequence but path expects a mapping (since next is key "b")
		err := yaml.WriteNode(&doc, yaml.PathOf("a", "b"), 1)
		assert.Error(err)
		assert.ErrorIs(err, yaml.ErrWriteCollision)
	})

	t.Run("collision sequence element is scalar", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("a:\n  - x\n"), &doc))
		// Element at a[0] is scalar but path expects a mapping (since next is key "b")
		err := yaml.WriteNode(&doc, yaml.PathOf("a", 0, "b"), 1)
		assert.Error(err)
		assert.ErrorIs(err, yaml.ErrWriteCollision)
	})

	t.Run("wildcard returns error", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("{}\n"), &doc))
		err := yaml.WriteNode(&doc, yaml.PathOf("*"), 1)
		assert.Error(err)
		assert.ErrorIs(err, yaml.ErrWritePath)

		err = yaml.WriteNode(&doc, yaml.PathOf("**"), 1)
		assert.Error(err)
		assert.ErrorIs(err, yaml.ErrWritePath)
	})

	t.Run("string value", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("a: old\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("a"), "new"))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf("a"))
		assert.True(ok)
		var val string
		Check(result.Decode(&val))
		assert.Equal("new", val)
	})

	t.Run("bool value", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("{}\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("enabled"), true))

		result, ok := yaml.MatchOne(&doc, yaml.PathOf("enabled"))
		assert.True(ok)
		var val bool
		Check(result.Decode(&val))
		assert.True(val)
	})

	t.Run("write to sequence", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("[]\n"), &doc))
		Check(yaml.WriteNode(&doc, yaml.PathOf(0, 0), 42))
		var val [][]int
		Check(doc.Decode(&val))
		assert.Equal([][]int{{42}}, val)
	})

	t.Run("alias intermediate", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("a: &anchor\n  x: 1\nb: *anchor\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("b", "x"), 99))

		// Both paths see the update since they share the anchor
		result, ok := yaml.MatchOne(&doc, yaml.PathOf("a", "x"))
		assert.True(ok)
		var val int
		Check(result.Decode(&val))
		assert.Equal(99, val)

		result, ok = yaml.MatchOne(&doc, yaml.PathOf("b", "x"))
		assert.True(ok)
		Check(result.Decode(&val))
		assert.Equal(99, val)
	})

	t.Run("alias leaf", func(t *testing.T) {
		assert := assert.New(t)
		var doc yamlv3.Node
		Check(yamlv3.Unmarshal([]byte("greeting: &val hello\nalias: *val\n"), &doc))
		assert.Nil(yaml.WriteNode(&doc, yaml.PathOf("alias"), "world"))

		// Both paths see the update since they share the anchor
		result, ok := yaml.MatchOne(&doc, yaml.PathOf("greeting"))
		assert.True(ok)
		var val string
		Check(result.Decode(&val))
		assert.Equal("world", val)

		result, ok = yaml.MatchOne(&doc, yaml.PathOf("alias"))
		assert.True(ok)
		Check(result.Decode(&val))
		assert.Equal("world", val)
	})
}
