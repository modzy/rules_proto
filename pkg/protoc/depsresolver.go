package protoc

import (
	"errors"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const (
	ResolverLangName = "protobuf"
	// ResolverImpLangPrivateKey stores the implementation language override.
	ResolverImpLangPrivateKey = "_protobuf_imp_lang"
	UnresolvedDepsPrivateKey  = "_unresolved_deps"
)

var (
	errSkipImport = errors.New("self import")
	errNotFound   = errors.New("rule not found")
	ErrNoLabel    = errors.New("no label")
)

type DepsResolver func(c *config.Config, ix *resolve.RuleIndex, r *rule.Rule, imports []string, from label.Label)

// ResolveDepsAttr returns a function that implements the DepsResolver
// interface.  This function resolves dependencies for a given rule attribute
// name (typically "deps"); if there is a non-empty list of resolved
// dependencies, the rule attribute will be overrwritten/modified by this
// function (excluding duplicates, sorting applied).  The "from" argument
// represents the rule being resolved (whose state is the *rule.Rule argument).
// The "imports" list represents the list of imports that was originally
// returned by the GenerateResponse.Imports (typically in via a private attr
// GazelleImportsKey), and holds the values of all the import statements (e.g.
// "google/protobuf/descriptor.proto") of the ProtoLibrary used to generate the
// rule.  Special handling is provided for well-known types, which can be
// excluded using the `excludeWkt` argument.  Actual resolution for an
// individual import is delegated to the `resolveAnyKind` function.
func ResolveDepsAttr(attrName string, excludeWkt bool) DepsResolver {
	return func(c *config.Config, ix *resolve.RuleIndex, r *rule.Rule, imports []string, from label.Label) {
		debug := false

		if debug {
			log.Printf("%v (%s.%s): resolving %d imports: %v", from, r.Kind(), attrName, len(imports), imports)
		}

		existing := r.AttrStrings(attrName)
		r.DelAttr(attrName)

		depSet := make(map[string]bool)
		for _, d := range existing {
			depSet[d] = true
		}

		// unresolvedDeps is a mapping from the import string to the reason it
		// was unresolved.  It is saved under 'UnresolvedDepsPrivateKey' if
		// there were unresolved deps.  The value 'ErrNoLabel' is the most
		// common case.
		unresolvedDeps := make(map[string]error)

		for _, imp := range imports {
			if excludeWkt && strings.HasPrefix(imp, "google/protobuf/") {
				continue
			}

			// determine the resolve kind
			impLang := r.Kind()
			if overrideImpLang, ok := r.PrivateAttr(ResolverImpLangPrivateKey).(string); ok {
				impLang = overrideImpLang
			}

			if debug {
				log.Println(from, "resolving:", imp, impLang)
			}
			l, err := resolveAnyKind(c, ix, ResolverLangName, impLang, imp, from)
			if err == errSkipImport {
				if debug {
					log.Println(from, "skipped (errSkipImport):", imp)
				}
				continue
			}
			if err != nil {
				log.Println(from, "ResolveDepsAttr error:", err)
				unresolvedDeps[imp] = err
				continue
			}
			if l == label.NoLabel {
				if debug {
					log.Println(from, "no label", imp)
				}
				unresolvedDeps[imp] = ErrNoLabel
				continue
			}

			l = l.Rel(from.Repo, from.Pkg)
			if debug {
				log.Println(from, "resolved:", imp, "is provided by", l)
			}
			depSet[l.String()] = true
		}

		if len(depSet) > 0 {
			deps := make([]string, 0, len(depSet))
			for dep := range depSet {
				deps = append(deps, dep)
			}
			sort.Strings(deps)
			r.SetAttr(attrName, deps)
			if debug {
				log.Println(from, "resolved deps:", deps)
			}
		}

		if len(unresolvedDeps) > 0 {
			r.SetPrivateAttr(UnresolvedDepsPrivateKey, unresolvedDeps)
		}
	}
}

// resolveAnyKind answers the question "what bazel label provides a rule for the
// given import?" (having the same rule kind as the given rule argument).  The
// algorithm first consults the override list (configured either via gazelle
// resolve directives, or via a YAML config).  If no override is found, the
// RuleIndex is consulted, which contains all rules indexed by gazelle in the
// generation phase.   If no match is found, return label.NoLabel.
func resolveAnyKind(c *config.Config, ix *resolve.RuleIndex, lang, impLang, imp string, from label.Label) (label.Label, error) {
	if l, ok := resolve.FindRuleWithOverride(c, resolve.ImportSpec{Lang: impLang, Imp: imp}, lang); ok {
		// log.Println(from, "override hit:", l)
		return l, nil
	}
	if l, err := resolveWithIndex(c, ix, lang, impLang, imp, from); err == nil || err == errSkipImport {
		return l, err
	} else if err != errNotFound {
		return label.NoLabel, err
	}
	// // if debug {
	// log.Println(from, "fallback miss:", imp)
	// // }
	return label.NoLabel, nil
}

func resolveWithIndex(c *config.Config, ix *resolve.RuleIndex, lang, impLang, imp string, from label.Label) (label.Label, error) {
	matches := ix.FindRulesByImportWithConfig(c, resolve.ImportSpec{Lang: impLang, Imp: imp}, lang)
	if len(matches) == 0 {
		// log.Println(from, "no matches:", imp)
		return label.NoLabel, errNotFound
	}
	if len(matches) > 1 {
		return label.NoLabel, fmt.Errorf("multiple rules (%s and %s) may be imported with %q from %s", matches[0].Label, matches[1].Label, imp, from)
	}
	if matches[0].IsSelfImport(from) || isSameImport(c, from, matches[0].Label) {
		// log.Println(from, "self import:", imp)
		return label.NoLabel, errSkipImport
	}
	// log.Println(from, "FindRulesByImportWithConfig first match:", imp, matches[0].Label)
	return matches[0].Label, nil
}

// isSameImport returns true if the "from" and "to" labels are the same.  If the
// "to" label is not a canonical label (having a fully-qualified repo name), a
// canonical label is constructed for comparison using the config.RepoName.
func isSameImport(c *config.Config, from, to label.Label) bool {
	if from == to {
		return true
	}
	if to.Repo != "" {
		return false
	}
	canonical := label.New(c.RepoName, to.Pkg, to.Name)
	return from == canonical
}

// StripRel removes the rel prefix from a filename (if has matching prefix)
func StripRel(rel string, filename string) string {
	if !strings.HasPrefix(filename, rel) {
		return filename
	}
	filename = filename[len(rel):]
	return strings.TrimPrefix(filename, "/")
}

// ProtoLibraryImportSpecsForKind generates an ImportSpec for each file in the
// set of given proto_library.
func ProtoLibraryImportSpecsForKind(kind string, libs ...ProtoLibrary) []resolve.ImportSpec {
	specs := make([]resolve.ImportSpec, 0)
	for _, lib := range libs {
		specs = append(specs, ProtoFilesImportSpecsForKind(kind, lib.Files())...)
	}

	return specs
}

// ProtoLibraryImportSpecsForKind generates an ImportSpec for each file in the
// set of given proto_library.
func ProtoFilesImportSpecsForKind(kind string, files []*File) []resolve.ImportSpec {
	specs := make([]resolve.ImportSpec, 0)
	for _, file := range files {
		imp := path.Join(file.Dir, file.Basename)
		spec := resolve.ImportSpec{Lang: kind, Imp: imp}
		specs = append(specs, spec)
	}
	return specs
}
