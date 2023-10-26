package parser

type ParsedSchema struct {
	Entities   EntityMap `json:"entities" yaml:"entities"`
	SchemaHash string    `json:"schemaHash" yaml:"schemaHash"`
	Warnings   []string  `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type EntityMap map[string]*Entity

type Entity struct {
	Relations map[string]*Relation `json:"relations" yaml:"relations"`
	Metadata  Metadata             `json:"metadata" yaml:"metadata"`
}

type Relation struct {
	Type                   string          `json:"type" yaml:"type"`
	Metadata               Metadata        `json:"metadata" yaml:"metadata"`
	DownstreamPermissions  []RelationTuple `json:"downstreamPermissions,omitempty" yaml:"downstreamPermissions,omitempty"`
	AllowedDirectRelations []RelationTuple `json:"allowedDirectRelations,omitempty" yaml:"allowedDirectRelations,omitempty"`
}

type RelationTuple struct {
	Entity   string `json:"entity" yaml:"entity"`
	Relation string `json:"relation,omitempty" yaml:"relation,omitempty"`
}

type Metadata struct {
	Comment    string                 `json:"comment" yaml:"comment"`
	Attributes map[string]interface{} `json:"attributes" yaml:"attributes"`
}

// resolveRelationRecursive walks the entity tree and finds all relations that reference the relationTuple provided.
//
// If the relationTuple points directly to a relation then a single item is returned.
func (m EntityMap) resolveRelationRecursive(relationTuple RelationTuple) []*Relation {
	entity := m[relationTuple.Entity]
	if entity == nil {
		return nil
	}

	relation := entity.Relations[relationTuple.Relation]
	if relation == nil {
		return nil
	}

	// Best case scenario is we've found the relation we're looking for
	if relation.Type == "RELATION" {
		return []*Relation{relation}
	}

	// Worst case scenario is we've found a permission, so we're going to find any
	// relations that reference it.
	ret := []*Relation{}

	for entityName, entity := range m {
		for relationName, relation := range entity.Relations {
			for _, permission := range relation.DownstreamPermissions {
				if permission == relationTuple {
					ret = append(ret, m.resolveRelationRecursive(RelationTuple{entityName, relationName})...)
				}
			}
		}
	}

	return ret
}
