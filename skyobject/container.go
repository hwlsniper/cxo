package skyobject

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/cipher/encoder"

	"github.com/skycoin/cxo/data"
)

var (
	hrefTypeName      = typeName(reflect.TypeOf(cipher.SHA256{}))
	hrefArrayTypeName = typeName(reflect.TypeOf([]cipher.SHA256{}))

	dynamicHrefSchema = getSchema(DynamicHref{})
)

// A Container is a helper type to manage skyobjects. The container is not
// thread safe
type Container struct {
	db   *data.DB
	root *Root

	registry map[string]*Schema
}

// NewContainer creates new Container that will use provided database.
// The database must not be nil
func NewContainer(db *data.DB) (c *Container) {
	if db == nil {
		panic("NewContainer tooks in nil-db")
	}
	c = &Container{
		db:       db,
		registry: make(map[string]*Schema),
	}
	return
}

// Root returns root object or nil
func (c *Container) Root() *Root {
	return c.root
}

// SetRoot replaces existing root from given one if timespamp of the given root
// is greater. The given root must not be nil
func (c *Container) SetRoot(root *Root) (ok bool) {
	if c.root == nil {
		c.root, ok = root, true
		return
	}
	if c.root.Time < root.Time {
		c.root, ok = root, true
	}
	return
}

// Register schema of given interface with provided name
func (c *Container) Register(name string, i interface{}) {
	c.registry[name] = getSchema(i)
}

// Schema returns registered Schema by given name
func (c *Container) Schema(name string) (s *Schema, ok bool) {
	s, ok = c.registry[name]
	return
}

// Save serializes given object and sotres it in DB returning
// key of the object
func (c *Container) Save(i interface{}) cipher.SHA256 {
	return c.db.AddAutoKey(encoder.Serialize(i))
}

// SaveArray saves array of objects and retursn references to them
func (c *Container) SaveArray(i ...interface{}) (ch []cipher.SHA256) {
	if len(i) == 0 {
		return // nil
	}
	ch = make([]cipher.SHA256, 0, len(i))
	for _, x := range i {
		ch = append(ch, c.Save(x))
	}
	return
}

// Get all child references without any filters
func (c *Container) Childs(schema Schema,
	data []byte) (ch map[*Schema][]cipher.SHA256, err error) {

	ch = make(map[*Schema][]cipher.SHA256)

	for _, sf := range schema.Fields {
		var tag string
		if tag = skyobjectTag(sf); !strings.Contains(tag, "href") {
			continue
		}
		var s *Schema
		switch sf.Type {
		case hrefTypeName:
			var k cipher.SHA256
			s, k, err = c.singleHref(data, schema.Fields, sf.Name, tag)
			if err != nil {
				ch = nil
				return
			}
			ch[s] = append(ch[s], k)
		case hrefArrayTypeName:
			var ks []cipher.SHA256
			s, ks, err = c.arrayHref(data, schema.Fields, sf.Name, tag)
			if err != nil {
				ch = nil
				return
			}
			ch[s] = append(ch[s], ks...)
		}
	}

	return
}

// get vlaue of `skyobjet:"xxx"` tag or empty string
func skyobjectTag(sf encoder.StructField) string {
	return reflect.StructTag(sf.Tag).Get("skyobject")
}

// tagSchemaName returns schema name or empty string if there is no
// schema=xxx in given tag, it returns an error if given tag
// is invalid
func tagSchemaName(tag string) (s string, err error) {
	for _, p := range strings.Split(tag, ",") {
		if strings.HasPrefix(p, "schema=") {
			ss := strings.Split(p, "=")
			if len(ss) != 2 {
				err = fmt.Errorf("invalid schema tag: %s", p)
				return
			}
			s = ss[1]
			return
		}
	}
	return
}

// if given tag contains schema=xxx then it reutrns appropriate schema or
// error if the schema is not registered, otherwise it returns
// dynamicHrefSchema
func (c *Container) schemaByTag(tag string) (schema *Schema, err error) {
	var (
		schemaName string
		ok         bool
	)
	if schemaName, err = tagSchemaName(tag); err != nil {
		return
	}
	if schemaName == "" { // DynamicHref
		schema = dynamicHrefSchema
	} else { //static href
		if schema, ok = c.registry[schemaName]; !ok {
			err = fmt.Errorf("unregistered schema: %s", schemaName)
		}
	}
	return
}

// extract href and schema from field, type of which is cipher.SHA256
func (c *Container) singleHref(data []byte, fields []encoder.StructField,
	fieldName, tag string) (schema *Schema, obj cipher.SHA256, err error) {

	if schema, err = c.schemaByTag(tag); err != nil {
		return
	}
	err = encoder.DeserializeField(data, fields, fieldName, &obj)
	return

}

// same as singleHref for []cipher.SHA256
func (c *Container) arrayHref(data []byte, fields []encoder.StructField,
	fieldName, tag string) (schema *Schema, objs []cipher.SHA256, err error) {

	if schema, err = c.schemaByTag(tag); err != nil {
		return
	}
	err = encoder.DeserializeField(data, fields, fieldName, &objs)
	return

}
