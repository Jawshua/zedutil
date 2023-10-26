# zedutil

A small utility app for working with [Authzed/SpiceDB schema files](https://authzed.com/docs/reference/schema-lang).

Usage:

```bash
# Run from source
go install github.com/Jawshua/zedutil@latest
zedutil ...

# Run from GitHub Releases
# https://github.com/Jawshua/zedutil/releases/latest
curl -L -o zedutil.tgz https://github.com/Jawshua/zedutil/releases/download/v0.1.1/zedutil_0.1.1_`uname | tr '[:upper:]' '[:lower:]'`_`uname -m`.tar.gz
tar xf zedutil.tgz zedutil
rm zedutil.tgz
./zedutil ...

# Run with Docker
docker run --rm -it ghcr.io/jawshua/zedutil ...
```

## genmap

`zedutil genmap` parses a .zed schema file and outputs a relation map file, which may be used for downstream code/doc generation.

zedutil relation map files provide the following:
* All top-level definitions as a key/value mapping.
* All relations and permissions contained within a definition as a key/value mapping.
* A list of all downstream permissions that a relation ultimately resolves to.
* A list of all entity types that are allowed to be added as a relation.
* Metadata for all definitions, relations, and permissions, including:
  * The comment attached to the entity
  * Custom attribute annotations, which are parsed from lines starting with `@attr`
    * `@attr foo=bar flag1 flag2` translates to `{"attributes": {"foo": "bar", "flag1": true, "flag2": true}}`

Example commands:
* `zedutil genmap example/simple-rbac/schema.zed` - generate a JSON map and write to stdout
* `zedutil genmap example/simple-rbac/schema.zed -f yaml` - generate a YAML map and write to stdout
* `zedutil genmap example/simple-rbac/schema.zed -o example/simple-rbac/map.json` - generate a JSON map and write to disk
* `zedutil genmap example/simple-rbac/schema.zed -o example/simple-rbac/map.yaml` - generate a YAML map and write to disk
* `zedutil genmap example/simple-rbac/schema.zed -q` - don't write parser warnings to stdout

Known limitations:
* Permissions defined using exclusions (`-` operator) or intersections (`&` operator) are not resolved, and will not appear in the `downstreamPermissions` list of a relation.


### Map Validation

The relation map output contains a sha256 hash of the schema file in the `schemaHash` attribute.
This can be used to validate that a map file matches the schema file.

Example:
```bash
if test `jq -r .schemaHash example/simple-rbac/map.json` != `shasum -a 256 example/simple-rbac/schema.zed | cut -d " " -f1`;
then
    echo "relation map is out of sync :("
fi
```

### Example Map

The following is a basic .zed schema that covers the features of the parser:

```authzed
/** @attr public app=auth
 * user represents a user that can be granted role(s)
 */
definition user {}

/** @attr public app=auth
 * group represents a group of users
 */
definition group {
    /**
     * direct_member is a direct member of the group
     */
    relation direct_member: user | user:*

    /**
     * member combines all users that may be considered a member of the group
     */
    permission member = direct_member
}

/** @attr public app=docstore
 * drive represents a drive protected by Authzed.
 */
definition drive {
    /** @attr public
     * owner indicates that the user is the owner of the drive.
     */
    relation owner: user | group#member

    /** @attr public
     * edit indicates that the user has permission to edit the drive.
     */
    permission edit = owner
}

/** @attr public app=docstore
 * document represents a document protected by Authzed.
 */
definition document {
    /** @attr public
     * drive is the drive that contains the document.
     */
    relation drive: drive

    /** @attr public
     * writer indicates that the user is a writer on the document.
     */
    relation writer: user | group#member

    /** @attr public
     * reader indicates that the user is a reader on the document.
     */
    relation reader: user | group#member

    /** @attr public
     * edit indicates that the user has permission to edit the document.
     */
    permission edit = writer + drive->edit

    /** @attr public
     * view indicates that the user has permission to view the document, if they
     * are a `reader` *or* have `edit` permission.
     */
    permission view = reader + edit
}
```

The resulting relation map looks something like this:

```yaml
entities:
  document:
    relations:
      drive:
        type: RELATION
        metadata:
          comment: drive is the drive that contains the document.
          attributes:
            public: true
        allowedDirectRelations:
          - entity: drive
      edit:
        type: PERMISSION
        metadata:
          comment: edit indicates that the user has permission to edit the document.
          attributes:
            public: true
      reader:
        type: RELATION
        metadata:
          comment: reader indicates that the user is a reader on the document.
          attributes:
            public: true
        downstreamPermissions:
          - entity: document
            relation: view
        allowedDirectRelations:
          - entity: user
          - entity: group
            relation: member
      view:
        type: PERMISSION
        metadata:
          comment: |-
            view indicates that the user has permission to view the document, if they
            are a `reader` *or* have `edit` permission.
          attributes:
            public: true
      writer:
        type: RELATION
        metadata:
          comment: writer indicates that the user is a writer on the document.
          attributes:
            public: true
        downstreamPermissions:
          - entity: document
            relation: edit
          - entity: document
            relation: view
        allowedDirectRelations:
          - entity: user
          - entity: group
            relation: member
    metadata:
      comment: document represents a document protected by Authzed.
      attributes:
        app: docstore
        public: true
  drive:
    relations:
      edit:
        type: PERMISSION
        metadata:
          comment: edit indicates that the user has permission to edit the drive.
          attributes:
            public: true
      owner:
        type: RELATION
        metadata:
          comment: owner indicates that the user is the owner of the drive.
          attributes:
            public: true
        downstreamPermissions:
          - entity: drive
            relation: edit
          - entity: document
            relation: edit
          - entity: document
            relation: view
        allowedDirectRelations:
          - entity: user
          - entity: group
            relation: member
    metadata:
      comment: drive represents a drive protected by Authzed.
      attributes:
        app: docstore
        public: true
  group:
    relations:
      direct_member:
        type: RELATION
        metadata:
          comment: direct_member is a direct member of the group
          attributes: {}
        downstreamPermissions:
          - entity: group
            relation: member
        allowedDirectRelations:
          - entity: user
          - entity: user
            relation: '*'
      member:
        type: PERMISSION
        metadata:
          comment: member combines all users that may be considered a member of the group
          attributes: {}
    metadata:
      comment: group represents a group of users
      attributes:
        app: auth
        public: true
  user:
    relations: {}
    metadata:
      comment: user represents a user that can be granted role(s)
      attributes:
        app: auth
        public: true
schemaHash: f6f6fd585dd1f79ecda908f9ddd55709c156c9b200bac54fd3314c7419226b36
```
