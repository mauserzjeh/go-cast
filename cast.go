package cast

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	castMagic uint32 = 0x74736163
)

var (
	castHashBase uint64 = 0x534E495752545250

	ErrEmptyValues = errors.New("cast: empty values")
)

// ----------------------- //
//          FILE           //
// ----------------------- //

// castHeader holds header data of the cast file
type castHeader struct {
	Magic     uint32
	Version   uint32
	RootNodes uint32
	Flags     uint32
}

// castFile holds data of a cast file
type castFile struct {
	flags     uint32
	version   uint32
	rootNodes []*castNode
}

// New creates a new [castFile]
func New() *castFile {
	return &castFile{
		flags:     0,
		version:   0x1,
		rootNodes: make([]*castNode, 0),
	}
}

// Load loads a [castFile] from the given [io.Reader]
func Load(r io.Reader) (*castFile, error) {
	var header castHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Magic != castMagic {
		return nil, fmt.Errorf("invalid cast file magic: %#x", header.Magic)
	}

	castFile := &castFile{
		flags:     header.Flags,
		version:   header.Version,
		rootNodes: make([]*castNode, header.RootNodes),
	}

	for i := range castFile.rootNodes {
		castFile.rootNodes[i] = &castNode{}
		if err := castFile.rootNodes[i].load(r); err != nil {
			return nil, err
		}
	}
	return castFile, nil
}

// Flags returns the flags
func (n *castFile) Flags() uint32 {
	return n.flags
}

// SetFlags sets the flags
func (n *castFile) SetFlags(flags uint32) *castFile {
	n.flags = flags
	return n
}

// Version returns the version
func (n *castFile) Version() uint32 {
	return n.version
}

// SetVersion sets the version
func (n *castFile) SetVersion(version uint32) *castFile {
	n.version = version
	return n
}

// Roots returns the root nodes
func (n *castFile) Roots() []*castNode {
	return n.rootNodes
}

// CreateRoot creates a root node
func (n *castFile) CreateRoot() *castNode {
	root := newCastNode(NodeIdRoot)
	n.rootNodes = append(n.rootNodes, root)
	return root
}

// Write writes the file to the given [io.Writer]
func (n *castFile) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, castHeader{
		Magic:     castMagic,
		Version:   n.version,
		RootNodes: uint32(len(n.rootNodes)),
		Flags:     n.flags,
	}); err != nil {
		return err
	}

	for _, rootNode := range n.rootNodes {
		if err := rootNode.write(w); err != nil {
			return err
		}
	}

	return nil
}

// ----------------------- //
//          NODE           //
// ----------------------- //

// CastNodeId type alias
type CastNodeId uint32

const (
	NodeIdRoot              CastNodeId = 0x746F6F72
	NodeIdModel             CastNodeId = 0x6C646F6D
	NodeIdMesh              CastNodeId = 0x6873656D
	NodeIdBlendShape        CastNodeId = 0x68736C62
	NodeIdSkeleton          CastNodeId = 0x6C656B73
	NodeIdBone              CastNodeId = 0x656E6F62
	NodeIdIKHandle          CastNodeId = 0x64686B69
	NodeIdConstraint        CastNodeId = 0x74736E63
	NodeIdAnimation         CastNodeId = 0x6D696E61
	NodeIdCurve             CastNodeId = 0x76727563
	NodeIdNotificationTrack CastNodeId = 0x6669746E
	NodeIdMaterial          CastNodeId = 0x6C74616D
	NodeIdFile              CastNodeId = 0x656C6966
	NodeIdInstance          CastNodeId = 0x74736E69
)

// castNodeHeader hold header data of a node
type castNodeHeader struct {
	Id            CastNodeId
	NodeSize      uint32
	NodeHash      uint64
	PropertyCount uint32
	ChildCount    uint32
}

// castNode holds data of a node
type castNode struct {
	id         CastNodeId
	nodeHash   uint64
	properties map[CastPropertyName]iCastProperty
	childNodes []*castNode
	parentNode *castNode
}

func newCastNode(id CastNodeId) *castNode {
	return &castNode{
		id:         id,
		nodeHash:   nextHash(),
		properties: map[CastPropertyName]iCastProperty{},
		childNodes: []*castNode{},
		parentNode: nil,
	}
}

// Id returns the id
func (n *castNode) Id() CastNodeId {
	return n.id
}

// Hash returns the hash
func (n *castNode) Hash() uint64 {
	return n.nodeHash
}

// SetParentNode sets the parent node
func (n *castNode) SetParentNode(node *castNode) {
	n.parentNode = node
}

// GetParentNode returns the parent node
func (n *castNode) GetParentNode() *castNode {
	return n.parentNode
}

// len returns the size of the node
func (n *castNode) len() int {
	l := 0x18

	for _, p := range n.properties {
		l += p.len()
	}

	for _, c := range n.childNodes {
		l += c.len()
	}
	return l
}

// load loads a node from the given [io.Reader]
func (n *castNode) load(r io.Reader) error {
	var header castNodeHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return err
	}

	if n.properties == nil {
		n.properties = make(map[CastPropertyName]iCastProperty)
	}

	for range header.PropertyCount {
		property, err := loadCastProperty(r)
		if err != nil {
			return err
		}

		n.properties[property.Name()] = property
	}

	n.childNodes = make([]*castNode, header.ChildCount)
	for i := range n.childNodes {
		n.childNodes[i] = &castNode{}
		if err := n.childNodes[i].load(r); err != nil {
			return err
		}
		n.childNodes[i].SetParentNode(n)
	}

	return nil
}

// write writes the node to the given [io.Writer]
func (n *castNode) write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, castNodeHeader{
		Id:            n.id,
		NodeSize:      uint32(n.len()),
		NodeHash:      n.nodeHash,
		PropertyCount: uint32(len(n.properties)),
		ChildCount:    uint32(len(n.childNodes)),
	}); err != nil {
		return err
	}

	for _, p := range n.properties {
		if err := p.write(w); err != nil {
			return err
		}
	}

	for _, c := range n.childNodes {
		if err := c.write(w); err != nil {
			return err
		}
	}

	return nil
}

// GetProperties returns the properties
func (n *castNode) GetProperties() map[CastPropertyName]iCastProperty {
	return n.properties
}

// GetProperty returns the property with the given name
func (n *castNode) GetProperty(name CastPropertyName) (iCastProperty, bool) {
	property, ok := n.properties[name]
	return property, ok
}

// CreateProperty creates a new property with the given name and type
func (n *castNode) CreateProperty(id CastPropertyId, name CastPropertyName) (iCastProperty, error) {
	property, err := newCastProperty(id, name, 0)
	if err != nil {
		return nil, err
	}

	if n.properties == nil {
		n.properties = make(map[CastPropertyName]iCastProperty)
	}

	n.properties[name] = property
	return property, nil
}

// GetChildNodes returns the child nodes
func (n *castNode) GetChildNodes() []*castNode {
	return n.childNodes
}

// GetChildrenOfType returns childnodes with the given type
func (n *castNode) GetChildrenOfType(id CastNodeId) []*castNode {
	nodes := make([]*castNode, 0)
	for _, c := range n.childNodes {
		if c.Id() == id {
			nodes = append(nodes, c)
		}
	}

	return nodes
}

// GetChildByHash returns a childnode with the given hash
func (n *castNode) GetChildByHash(hash uint64) *castNode {
	for _, c := range n.childNodes {
		if c.Hash() == hash {
			return c
		}
	}

	return nil
}

// CreateChild creates a new childnode
func (n *castNode) CreateChild(id CastNodeId) *castNode {
	child := newCastNode(id)
	child.SetParentNode(n)
	n.childNodes = append(n.childNodes, child)
	return child
}

// ----------------------- //
//       PROPERTIES        //
// ----------------------- //

type CastPropertyId uint16

const (
	PropByte      CastPropertyId = 'b'
	PropShort     CastPropertyId = 'h'
	PropInteger32 CastPropertyId = 'i'
	PropInteger64 CastPropertyId = 'l'
	PropFloat     CastPropertyId = 'f'
	PropDouble    CastPropertyId = 'd'
	PropString    CastPropertyId = 's'
	PropVector2   CastPropertyId = 0x7632
	PropVector3   CastPropertyId = 0x7633
	PropVector4   CastPropertyId = 0x7634
)

type CastPropertyName string

const (
	PropNameName                   CastPropertyName = "n"
	PropNameVertexPositionBuffer   CastPropertyName = "vp"
	PropNameVertexNormalBuffer     CastPropertyName = "vn"
	PropNameVertexTangentBuffer    CastPropertyName = "vt"
	PropNameVertexColorBuffer      CastPropertyName = "vc"
	PropNameVertexUVBuffer         CastPropertyName = "u%d"
	PropNameVertexWeightBoneBuffer CastPropertyName = "wv"
	PropNameFaceBuffer             CastPropertyName = "f"
	PropNameUVLayerCount           CastPropertyName = "ul"
	PropNameMaximumWeightInfluence CastPropertyName = "mi"
	PropNameSkinningMethod         CastPropertyName = "sm"
	PropNameMaterial               CastPropertyName = "m"
	PropNameBaseShape              CastPropertyName = "b"
	PropNameTargetShape            CastPropertyName = "t"
	PropNameTargetWeightScale      CastPropertyName = "ts"
	PropNameParentIndex            CastPropertyName = "p"
	PropNameSegmentScaleCompensate CastPropertyName = "ssc"
	PropNameLocalPosition          CastPropertyName = "lp"
	PropNameLocalRotation          CastPropertyName = "lr"
	PropNameWorldPosition          CastPropertyName = "wp"
	PropNameWorldRotation          CastPropertyName = "wr"
	PropNameScale                  CastPropertyName = "s"
	PropNameStartBone              CastPropertyName = "sb"
	PropNameEndBone                CastPropertyName = "eb"
	PropNameTargetBone             CastPropertyName = "tb"
	PropNamePoleVectorBone         CastPropertyName = "pv"
	PropNamePoleBone               CastPropertyName = "pb"
	PropNameTargetRotation         CastPropertyName = "tr"
	PropNameConstraintType         CastPropertyName = "ct"
	PropNameConstraintBone         CastPropertyName = "cb"
	PropNameMaintainOffset         CastPropertyName = "mo"
	PropNameSkipX                  CastPropertyName = "sx"
	PropNameSkipY                  CastPropertyName = "sy"
	PropNameSkipZ                  CastPropertyName = "sz"
	PropNameType                   CastPropertyName = "t"
	PropNamePath                   CastPropertyName = "p"
	PropNameFramerate              CastPropertyName = "fr"
	PropNameLoop                   CastPropertyName = "lo"
	PropNameNodeName               CastPropertyName = "nn"
	PropNameKeyProperty            CastPropertyName = "kp"
	PropNameKeyFrameBuffer         CastPropertyName = "kb"
	PropNameKeyValueBuffer         CastPropertyName = "kv"
	PropNameMode                   CastPropertyName = "m"
	PropNameAdditiveBlendWeight    CastPropertyName = "ab"
	PropNameReferenceFile          CastPropertyName = "rf"
	PropNamePosition               CastPropertyName = "p"
	PropNameRotation               CastPropertyName = "r"
)

// castPropertyHeader holds header data of the property
type castPropertyHeader struct {
	Id          CastPropertyId
	NameSize    uint16
	ArrayLength uint32
}

// iCastProperty is the property interface
type iCastProperty interface {
	Id() CastPropertyId
	Name() CastPropertyName
	ValueCount() int
	len() int
	load(r io.Reader) error
	write(w io.Writer) error
}

type CastPropertyValueType interface {
	byte | uint16 | uint32 | uint64 | float32 | float64 | string | Vec2 | Vec3 | Vec4
}

// castProperty holds data of a property
type castProperty[T CastPropertyValueType] struct {
	id     CastPropertyId
	name   CastPropertyName
	values []T
}

// Id returns the property id
func (p *castProperty[T]) Id() CastPropertyId {
	return p.id
}

// Name returns the name
func (p *castProperty[T]) Name() CastPropertyName {
	return p.name
}

// ValueCount returns the amount of values held by the property
func (p *castProperty[T]) ValueCount() int {
	return len(p.values)
}

// Values returns the values held by the property
func (p *castProperty[T]) Values() []T {
	return p.values
}

// Length returns the length of the property
func (p *castProperty[T]) len() int {
	l := 0x8

	l += len(p.name)
	switch vs := any(p.values).(type) {
	case []string:
		l += len(vs[0]) + 1
	default:
		l += binary.Size(p.values)
	}

	return l
}

// load loads a property from the given [io.Reader]
func (p *castProperty[T]) load(r io.Reader) error {
	switch any(p.values).(type) {
	case []string:
		str, err := readString(r)
		if err != nil {
			return err
		}

		p.values = any([]string{str}).([]T)
		return nil
	default:
		return binary.Read(r, binary.LittleEndian, &p.values)
	}
}

// write writes a property to the given [io.Writer]
func (p *castProperty[T]) write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, castPropertyHeader{
		Id:          p.id,
		NameSize:    uint16(len(p.name)),
		ArrayLength: uint32(binary.Size(p.values)),
	}); err != nil {
		return err
	}

	if _, err := w.Write([]byte(p.name)); err != nil {
		return err
	}

	switch vs := any(p.values).(type) {
	case []string:
		s := []byte(vs[0] + "\x00")
		if err := binary.Write(w, binary.LittleEndian, s); err != nil {
			return err
		}
	default:
		if err := binary.Write(w, binary.LittleEndian, p.values); err != nil {
			return err
		}
	}

	return nil
}

// newCastProperty creates a new property with the given type, name and size
func newCastProperty(id CastPropertyId, name CastPropertyName, size uint32) (iCastProperty, error) {
	switch id {
	case PropByte:
		return &castProperty[byte]{
			id:     id,
			name:   name,
			values: make([]byte, size),
		}, nil
	case PropShort:
		return &castProperty[uint16]{
			id:     id,
			name:   name,
			values: make([]uint16, size),
		}, nil
	case PropInteger32:
		return &castProperty[uint32]{
			id:     id,
			name:   name,
			values: make([]uint32, size),
		}, nil
	case PropInteger64:
		return &castProperty[uint64]{
			id:     id,
			name:   name,
			values: make([]uint64, size),
		}, nil
	case PropFloat:
		return &castProperty[float32]{
			id:     id,
			name:   name,
			values: make([]float32, size),
		}, nil
	case PropDouble:
		return &castProperty[float64]{
			id:     id,
			name:   name,
			values: make([]float64, size),
		}, nil
	case PropString:
		return &castProperty[string]{
			id:     id,
			name:   name,
			values: make([]string, size),
		}, nil
	case PropVector2:
		return &castProperty[Vec2]{
			id:     id,
			name:   name,
			values: make([]Vec2, size),
		}, nil
	case PropVector3:
		return &castProperty[Vec3]{
			id:     id,
			name:   name,
			values: make([]Vec3, size),
		}, nil

	case PropVector4:
		return &castProperty[Vec4]{
			id:     id,
			name:   name,
			values: make([]Vec4, size),
		}, nil
	default:
		return nil, fmt.Errorf("cast: invalid property id: %#x", id)
	}
}

// loadCastProperty loads a property from the given [io.Reader]
func loadCastProperty(r io.Reader) (iCastProperty, error) {
	var header castPropertyHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	var name = make([]byte, header.NameSize)
	if err := binary.Read(r, binary.LittleEndian, &name); err != nil {
		return nil, err
	}

	property, err := newCastProperty(header.Id, CastPropertyName(name), header.ArrayLength)
	if err != nil {
		return nil, err
	}

	if err := property.load(r); err != nil {
		return nil, err
	}

	return property, nil
}

// CreateProperty creates a new property on the given node with the given values
func CreateProperty[T CastPropertyValueType](node *castNode, name CastPropertyName, id CastPropertyId, values ...T) (*castProperty[T], error) {
	property, err := node.CreateProperty(id, name)
	if err != nil {
		return nil, err
	}
	p := property.(*castProperty[T])
	p.values = append(p.values, values...)
	return p, nil
}

// GetPropertyValues returns the property values of the given node
func GetPropertyValues[T CastPropertyValueType](node *castNode, name CastPropertyName) ([]T, error) {
	property, ok := node.GetProperty(name)
	if !ok {
		return nil, fmt.Errorf(`cast: property %s not found`, name)
	}

	p, ok := property.(*castProperty[T])
	if !ok {
		return nil, fmt.Errorf("cast: property has a type of %T instead of %T", property, &castProperty[T]{})
	}

	return p.values, nil
}

// GetPropertyValue returns a pointer to the first property value of the given node
func GetPropertyValue[T CastPropertyValueType](node *castNode, name CastPropertyName) (*T, error) {
	values, err := GetPropertyValues[T](node, name)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, ErrEmptyValues
	}
	return &values[0], nil
}

// ----------------------- //
//         HELPERS         //
// ----------------------- //

// readString reads a null terminated string from the given [io.Reader]
func readString(r io.Reader) (string, error) {
	str := []byte{}

	for {
		var b byte
		err := binary.Read(r, binary.LittleEndian, &b)
		if err != nil && err != io.EOF {
			return "", err
		}

		if b == 0 {
			break
		}

		str = append(str, b)
	}

	return string(str), nil
}

// nextHash returns the next hash
func nextHash() uint64 {
	hash := castHashBase
	castHashBase++
	return hash
}

// Vec2 is a structure holding data of a Vector2
type Vec2 struct {
	X, Y float32
}

// Vec3 is a structure holding data of a Vector3
type Vec3 struct {
	X, Y, Z float32
}

// Vec4 is a structure holding data of a Vector4
type Vec4 struct {
	X, Y, Z, W float32
}
