package parser

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strings"

	corev1 "github.com/authzed/spicedb/pkg/proto/core/v1"
	implv1 "github.com/authzed/spicedb/pkg/proto/impl/v1"
	"github.com/authzed/spicedb/pkg/schemadsl/compiler"
	"github.com/authzed/spicedb/pkg/schemadsl/input"
)

var (
	attributeRegex = regexp.MustCompile(`(?mi)^[ ]*@attr([ ]+[\w -=]+)$`)
	commentRegex   = regexp.MustCompile(`(?mi)^[ \t/*]+`)
)

// Parse parses the provided zedFile and returns a mapping of the definitions it contains.
//
// err will be non-nil if an error occurs while opening or compiling the schema file
func Parse(zedFile string) (ret *ParsedSchema, err error) {
	schemaData, err := os.ReadFile(zedFile)
	if err != nil {
		return nil, fmt.Errorf("error opening authzed schema file [%s]: %w", zedFile, err)
	}
	schemaDataString := string(schemaData)

	prefix := ""
	compiledSchema, err := compiler.Compile(compiler.InputSchema{
		Source:       input.Source(zedFile),
		SchemaString: schemaDataString,
	}, &prefix)

	if err != nil {
		return nil, fmt.Errorf("error compiling authzed schema [%s]: %w", zedFile, err)
	}

	entityMap := make(EntityMap)
	warnings := make([]string, 0)

	// Initial pass to construct the basic entity skeleton
	for _, ns := range compiledSchema.ObjectDefinitions {
		md, _ := extractMetadata(ns.Metadata)
		entityMap[ns.Name] = &Entity{
			Relations: make(map[string]*Relation),
			Metadata:  md,
		}

		for _, relation := range ns.Relation {
			md, relationType := extractMetadata(relation.Metadata)
			directRelations := make([]RelationTuple, 0, len(relation.TypeInformation.GetAllowedDirectRelations()))

			for _, directRelation := range relation.TypeInformation.GetAllowedDirectRelations() {
				switch x := directRelation.RelationOrWildcard.(type) {
				case *corev1.AllowedRelation_Relation:
					relation := x.Relation
					if relation == "..." {
						relation = ""
					}
					directRelations = append(directRelations, RelationTuple{
						Entity:   directRelation.Namespace,
						Relation: relation,
					})
				case *corev1.AllowedRelation_PublicWildcard_:
					directRelations = append(directRelations, RelationTuple{
						Entity:   directRelation.Namespace,
						Relation: "*",
					})
				}
			}

			entityMap[ns.Name].Relations[relation.Name] = &Relation{
				Metadata:               md,
				Type:                   relationType,
				AllowedDirectRelations: directRelations,
			}
		}
	}

	// Second pass to populate permissions granted by relations
	for _, ns := range compiledSchema.ObjectDefinitions {
		for _, relation := range ns.Relation {
			switch x := relation.UsersetRewrite.GetRewriteOperation().(type) {
			case *corev1.UsersetRewrite_Union:
				for _, item := range x.Union.GetChild() {
					var (
						referencedRelation RelationTuple
					)
					switch y := item.GetChildType().(type) {
					case *corev1.SetOperation_Child_ComputedUserset:
						referencedRelation = RelationTuple{ns.Name, y.ComputedUserset.GetRelation()}
					case *corev1.SetOperation_Child_TupleToUserset:
						referencedRelation = RelationTuple{y.TupleToUserset.Tupleset.Relation, y.TupleToUserset.ComputedUserset.Relation}
					}

					if referencedRelation.Entity == "" || referencedRelation.Relation == "" {
						warnings = append(warnings, fmt.Sprintf("%s->%s: could not determine reference tuple from union type: %v", ns.Name, relation.Name, item))
						continue
					}

					// Resolve the referenced computed userset relation, and any relations that reference it.
					relations := entityMap.resolveRelationRecursive(referencedRelation)

					// Add the relation as a permission reference to all of the relations
					for _, r := range relations {
						r.DownstreamPermissions = append(r.DownstreamPermissions, RelationTuple{ns.Name, relation.Name})
					}
				}
			case *corev1.UsersetRewrite_Intersection:
				warnings = append(warnings, fmt.Sprintf("%s->%s: intersections (& operator) are currently not supported", ns.Name, relation.Name))
			case *corev1.UsersetRewrite_Exclusion:
				warnings = append(warnings, fmt.Sprintf("%s->%s: exclusions (- operator) are currently not supported", ns.Name, relation.Name))
			}

		}
	}

	return &ParsedSchema{
		Entities:   entityMap,
		SchemaHash: fmt.Sprintf("%x", sha256.Sum256(schemaData)),
		Warnings:   warnings,
	}, nil
}

func extractMetadata(metadata *corev1.Metadata) (ret Metadata, relationType string) {
	ret = Metadata{
		Attributes: make(map[string]interface{}),
	}

	for _, md := range metadata.GetMetadataMessage() {
		mdProto, err := md.UnmarshalNew()
		if err != nil {
			panic(err)
		}

		switch x := mdProto.(type) {
		case *implv1.RelationMetadata:
			relationType = x.Kind.String()
		case *implv1.DocComment:
			ret.Comment = commentRegex.ReplaceAllString(x.Comment, "")
		}
	}

	for _, match := range attributeRegex.FindAllStringSubmatch(ret.Comment, -1) {
		if len(match) != 2 {
			continue
		}

		for _, attribute := range strings.Split(match[1], " ") {
			attribute = strings.TrimSpace(attribute)
			if attribute == "" {
				continue
			}

			pair := strings.SplitN(attribute, "=", 2)

			if len(pair) == 1 {
				ret.Attributes[pair[0]] = true
				continue
			}

			ret.Attributes[pair[0]] = pair[1]
		}
	}

	ret.Comment = attributeRegex.ReplaceAllString(ret.Comment, "")
	ret.Comment = strings.TrimSpace(ret.Comment)

	return
}
