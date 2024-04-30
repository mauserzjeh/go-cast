package cast

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	castMagic uint32 = 0x74736163
)

var (
	castHashBase uint64 = 0x534E495752545250
)

type CastFile struct {
	rootNodes []ICastNode
}

func Load(file string) (*CastFile, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var header CastHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Magic != castMagic {
		return nil, fmt.Errorf("invalid cast file magic: %#x", header.Magic)
	}

	castFile := &CastFile{
		rootNodes: make([]ICastNode, header.RootNodes),
	}

	for i := range castFile.rootNodes {
		castFile.rootNodes[i] = &CastNode{}
		if err := castFile.rootNodes[i].Load(f); err != nil {
			return nil, err
		}
	}
	return castFile, nil
}

type CastHeader struct {
	Magic     uint32
	Version   uint32
	RootNodes uint32
	Flags     uint32
}

type CastId uint32

const (
	Root              CastId = 0x746F6F72
	Model             CastId = 0x6C646F6D
	Mesh              CastId = 0x6873656D
	BlendShape        CastId = 0x68736C62
	Skeleton          CastId = 0x6C656B73
	Bone              CastId = 0x656E6F62
	IKHandle          CastId = 0x64686B69
	Constraint        CastId = 0x74736E63
	Animation         CastId = 0x6D696E61
	Curve             CastId = 0x76727563
	NotificationTrack CastId = 0x6669746E
	Material          CastId = 0x6C74616D
	File              CastId = 0x656C6966
	Instance          CastId = 0x74736E69
)

type CastNodeHeader struct {
	Identifier    CastId
	NodeSize      uint32
	NodeHash      uint64
	PropertyCount uint32
	ChildCount    uint32
}

type ICastNode interface {
	Identifier() CastId
	Hash() uint64
	Load(r io.Reader) error
	SetParentNode(node ICastNode)
	Lenght() int
	Write(w io.Writer) error
}

type CastNode struct {
	Id         CastId
	NodeHash   uint64
	Properties map[CastPropertyName]ICastProperty
	ChildNodes []ICastNode
	ParentNode ICastNode
}

func NewCastNode() *CastNode {
	return &CastNode{
		Id:         0,
		NodeHash:   nextHash(),
		Properties: make(map[CastPropertyName]ICastProperty),
		ChildNodes: make([]ICastNode, 0),
		ParentNode: nil,
	}
}

func (n *CastNode) Identifier() CastId {
	return n.Id
}

func (n *CastNode) Hash() uint64 {
	return n.NodeHash
}

func (n *CastNode) Load(r io.Reader) error {
	var header CastNodeHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return err
	}

	if n.Properties == nil {
		n.Properties = make(map[CastPropertyName]ICastProperty)
	}

	for range header.PropertyCount {
		property, err := loadCastProperty(r)
		if err != nil {
			return err
		}

		n.Properties[property.Name()] = property
	}

	n.ChildNodes = make([]ICastNode, header.ChildCount)
	for i := range n.ChildNodes {
		n.ChildNodes[i] = &CastNode{}
		if err := n.ChildNodes[i].Load(r); err != nil {
			return err
		}
		n.ChildNodes[i].SetParentNode(n)
	}

	return nil
}

func (n *CastNode) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, CastNodeHeader{
		Identifier:    n.Id,
		NodeSize:      uint32(n.Lenght()),
		NodeHash:      n.NodeHash,
		PropertyCount: uint32(len(n.Properties)),
		ChildCount:    uint32(len(n.ChildNodes)),
	}); err != nil {
		return err
	}

	for _, p := range n.Properties {
		if err := p.Write(w); err != nil {
			return err
		}
	}

	for _, c := range n.ChildNodes {
		if err := c.Write(w); err != nil {
			return err
		}
	}

	return nil
}

func (n *CastNode) SetParentNode(node ICastNode) {
	n.ParentNode = node
}

func (n *CastNode) Lenght() int {
	l := 0x18

	for _, p := range n.Properties {
		l += p.Length()
	}

	for _, c := range n.ChildNodes {
		l += c.Lenght()
	}
	return l
}

func (n *CastNode) GetProperty(name CastPropertyName) (ICastProperty, bool) {
	property, ok := n.Properties[name]
	return property, ok
}

func (n *CastNode) CreateProperty(name CastPropertyName, id CastPropertyId) (ICastProperty, error) {
	property, err := newCastProperty(id, name, 0)
	if err != nil {
		return nil, err
	}

	if n.Properties == nil {
		n.Properties = make(map[CastPropertyName]ICastProperty)
	}

	n.Properties[name] = property
	return property, nil
}

func (n *CastNode) ChildrenOfType(id CastId) []ICastNode {
	nodes := make([]ICastNode, 0)
	for _, c := range n.ChildNodes {
		if c.Identifier() == id {
			nodes = append(nodes, c)
		}
	}

	return nodes
}

func (n *CastNode) ChildByHash(hash uint64) ICastNode {
	nodes := make([]ICastNode, 0)
	for _, c := range n.ChildNodes {
		if c.Hash() == hash {
			nodes = append(nodes, c)
		}
	}

	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

func (n *CastNode) CreateChild(child ICastNode) ICastNode {
	child.SetParentNode(n)
	n.ChildNodes = append(n.ChildNodes, child)
	return child
}

type CastNodeModel struct{ CastNode }

func (n *CastNodeModel) Name() CastPropertyName {
	property, ok := n.GetProperty(CastPropertyNameName)
	if !ok {
		return ""
	}

	return property.Name()
}

func (n *CastNodeModel) SetName(name string) {
	createProperty(&n.CastNode, CastPropertyNameName, CastPropertyString, name)
}

func (n *CastNodeModel) Skeleton() *CastNodeSkeleton {
	skeletons := n.ChildrenOfType(Skeleton)
	if len(skeletons) > 0 {
		return (skeletons[0]).(*CastNodeSkeleton)
	}

	return nil
}

func (n *CastNodeModel) CreateSkeleton() *CastNodeSkeleton {
	return (n.CreateChild(&CastNodeSkeleton{})).(*CastNodeSkeleton)
}

func (n *CastNodeModel) Meshes() []*CastNodeMesh {
	ms := n.ChildrenOfType(Mesh)
	meshes := make([]*CastNodeMesh, len(ms))

	for i := range meshes {
		meshes[i] = (ms[i]).(*CastNodeMesh)
	}

	return meshes
}

func (n *CastNodeModel) CreateMesh() *CastNodeMesh {
	return (n.CreateChild(&CastNodeMesh{})).(*CastNodeMesh)
}

func (n *CastNodeModel) Materials() []*CastNodeMaterial {
	ms := n.ChildrenOfType(Material)
	materials := make([]*CastNodeMaterial, len(ms))

	for i := range materials {
		materials[i] = (ms[i]).(*CastNodeMaterial)
	}

	return materials
}

func (n *CastNodeModel) CreateMaterial() *CastNodeMaterial {
	return (n.CreateChild(&CastNodeMaterial{})).(*CastNodeMaterial)
}

func (n *CastNodeModel) BlendShapes() []*CastNodeBlendShape {
	bs := n.ChildrenOfType(BlendShape)
	blendShapes := make([]*CastNodeBlendShape, len(bs))

	for i := range blendShapes {
		blendShapes[i] = (bs[i]).(*CastNodeBlendShape)
	}

	return blendShapes
}

func (n *CastNodeModel) CreateBlendShape() *CastNodeBlendShape {
	return (n.CreateChild(&CastNodeBlendShape{})).(*CastNodeBlendShape)
}

type CastNodeMesh struct{ CastNode }

func (n *CastNodeMesh) Name() CastPropertyName {
	property, ok := n.GetProperty(CastPropertyNameName)
	if !ok {
		return ""
	}

	return property.Name()
}

func (n *CastNodeMesh) SetName(name string) {
	createProperty(&n.CastNode, CastPropertyNameName, CastPropertyString, name)
}

func (n *CastNodeMesh) VertexCount() int {
	property, ok := n.GetProperty(CastPropertyNameVertexPositionBuffer)
	if !ok {
		return 0
	}
	return property.ValueCount()
}

func (n *CastNodeMesh) FaceCount() int {
	property, ok := n.GetProperty(CastPropertyNameFaceBuffer)
	if !ok {
		return 0
	}
	return property.ValueCount() / 3
}

// TODO
func (n *CastNodeMesh) UVLayerCount() int {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameUVLayerCount)
	if len(values) == 0 {
		return 0
	}
	return int(values[0])
}

func (n *CastNodeMesh) SetUVLayerCount(count byte) {
	createProperty(&n.CastNode, CastPropertyNameUVLayerCount, CastPropertyByte, count)
}

// TODO
func (n *CastNodeMesh) MaximumWeightInfluence() int {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameMaximumWeightInfluence)
	if len(values) == 0 {
		return 0
	}
	return int(values[0])
}

func (n *CastNodeMesh) SetMaximumWeightInfluence(maximum byte) {
	createProperty(&n.CastNode, CastPropertyNameMaximumWeightInfluence, CastPropertyByte, maximum)
}

func (n *CastNodeMesh) SkinningMethod() string {
	values := getPropertyValues[string](&n.CastNode, CastPropertyNameSkinningMethod)
	if len(values) == 0 {
		return "linear"
	}
	return values[0]
}

func (n *CastNodeMesh) SetSkinningMethod(method string) {
	createProperty(&n.CastNode, CastPropertyNameSkinningMethod, CastPropertyString, method)
}

// TODO
func (n *CastNodeMesh) FaceBuffer() []int32 {
	return getPropertyValues[int32](&n.CastNode, CastPropertyNameFaceBuffer)
}

func (n *CastNodeMesh) SetFaceBuffer(values []int32) {
	createProperty(&n.CastNode, CastPropertyNameFaceBuffer, CastPropertyInteger32, values...)
}

func (n *CastNodeMesh) VertexPositionBuffer() []Vec3 {
	return getPropertyValues[Vec3](&n.CastNode, CastPropertyNameVertexPositionBuffer)
}

func (n *CastNodeMesh) SetVertexPositionBuffer(values []Vec3) {
	createProperty(&n.CastNode, CastPropertyNameVertexPositionBuffer, CastPropertyVector3, values...)
}

func (n *CastNodeMesh) VertexNormalBuffer() []Vec3 {
	return getPropertyValues[Vec3](&n.CastNode, CastPropertyNameVertexNormalBuffer)
}

func (n *CastNodeMesh) SetVertexNormalBuffer(values []Vec3) {
	createProperty(&n.CastNode, CastPropertyNameVertexNormalBuffer, CastPropertyVector3, values...)
}

func (n *CastNodeMesh) VertexTangentBuffer() []Vec3 {
	return getPropertyValues[Vec3](&n.CastNode, CastPropertyNameVertexTangentBuffer)
}

func (n *CastNodeMesh) SetVertexTangentBuffer(values []Vec3) {
	createProperty(&n.CastNode, CastPropertyNameVertexTangentBuffer, CastPropertyVector3, values...)
}

func (n *CastNodeMesh) VertexColorBuffer() []int32 {
	return getPropertyValues[int32](&n.CastNode, CastPropertyNameVertexColorBuffer)
}

func (n *CastNodeMesh) SetVertexColorBuffer(values []int32) {
	createProperty(&n.CastNode, CastPropertyNameVertexColorBuffer, CastPropertyInteger32, values...)
}

func (n *CastNodeMesh) VertexUVLayerBuffer(index int) []Vec2 {
	return getPropertyValues[Vec2](&n.CastNode, CastPropertyName(fmt.Sprintf(string(CastPropertyNameVertexUVBuffer), index)))
}

func (n *CastNodeMesh) SetVertexUVLayerBuffer(index int, values []Vec2) {
	createProperty(&n.CastNode, CastPropertyName(fmt.Sprintf(string(CastPropertyNameVertexUVBuffer), index)), CastPropertyVector2, values...)
}

// TODO
func (n *CastNodeMesh) VertexWeightBoneBuffer() []int32 {
	return getPropertyValues[int32](&n.CastNode, "wb")
}

func (n *CastNodeMesh) SetVertexWeightBoneBuffer(values []int32) {
	createProperty(&n.CastNode, "wb", CastPropertyInteger32, values...)
}

func (n *CastNodeMesh) VertexWeightValueBuffer() []float32 {
	return getPropertyValues[float32](&n.CastNode, CastPropertyNameVertexWeightBoneBuffer)
}

func (n *CastNodeMesh) SetVertexWeightValueBuffer(values []float32) {
	createProperty(&n.CastNode, CastPropertyNameVertexWeightBoneBuffer, CastPropertyFloat, values...)
}

func (n *CastNodeMesh) Material() *CastNodeMaterial {
	materialHashes := getPropertyValues[int64](&n.CastNode, CastPropertyNameMaterial)
	if len(materialHashes) == 0 {
		return nil
	}

	m := n.ChildByHash(uint64(materialHashes[0]))
	if m == nil {
		return nil
	}

	material, ok := m.(*CastNodeMaterial)
	if !ok {
		return nil
	}

	return material
}

func (n *CastNodeMesh) SetMaterial(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNameMaterial, CastPropertyInteger64, int64(hash))
}

type CastNodeBlendShape struct{ CastNode }        // TODO
type CastNodeSkeleton struct{ CastNode }          // TODO
type CastNodeBone struct{ CastNode }              // TODO
type CastNodeIKHandle struct{ CastNode }          // TODO
type CastNodeConstraint struct{ CastNode }        // TODO
type CastNodeMaterial struct{ CastNode }          // TODO
type CastNodeFile struct{ CastNode }              // TODO
type CastNodeAnimation struct{ CastNode }         // TODO
type CastNodeCurve struct{ CastNode }             // TODO
type CastNodeNotificationTrack struct{ CastNode } // TODO
type CastNodeInstance struct{ CastNode }          // TODO

type CastPropertyId uint16

const (
	CastPropertyByte      CastPropertyId = 'b'
	CastPropertyShort     CastPropertyId = 'h'
	CastPropertyInteger32 CastPropertyId = 'i'
	CastPropertyInteger64 CastPropertyId = 'l'
	CastPropertyFloat     CastPropertyId = 'f'
	CastPropertyDouble    CastPropertyId = 'd'
	CastPropertyString    CastPropertyId = 's'
	CastPropertyVector2   CastPropertyId = 0x7632
	CastPropertyVector3   CastPropertyId = 0x7633
	CastPropertyVector4   CastPropertyId = 0x7634
)

type CastPropertyName string

const (
	CastPropertyNameName                   CastPropertyName = "n"
	CastPropertyNameVertexPositionBuffer   CastPropertyName = "vp"
	CastPropertyNameVertexNormalBuffer     CastPropertyName = "vn"
	CastPropertyNameVertexTangentBuffer    CastPropertyName = "vt"
	CastPropertyNameVertexColorBuffer      CastPropertyName = "vc"
	CastPropertyNameVertexUVBuffer         CastPropertyName = "u%d"
	CastPropertyNameVertexWeightBoneBuffer CastPropertyName = "wv"
	CastPropertyNameFaceBuffer             CastPropertyName = "f"
	CastPropertyNameUVLayerCount           CastPropertyName = "ul"
	CastPropertyNameMaximumWeightInfluence CastPropertyName = "mi"
	CastPropertyNameSkinningMethod         CastPropertyName = "sm"
	CastPropertyNameMaterial               CastPropertyName = "m"
)

type CastPropertyHeader struct {
	Identifier  CastPropertyId
	NameSize    uint16
	ArrayLength uint32
}

type ICastProperty interface {
	Name() CastPropertyName
	Identifier() CastPropertyId
	Load(r io.Reader) error
	Length() int
	Write(w io.Writer) error
	ValueCount() int
}

func loadCastProperty(r io.Reader) (ICastProperty, error) {
	var header CastPropertyHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	var name = make([]byte, header.NameSize)
	if err := binary.Read(r, binary.LittleEndian, &name); err != nil {
		return nil, err
	}

	property, err := newCastProperty(header.Identifier, CastPropertyName(name), header.ArrayLength)
	if err != nil {
		return nil, err
	}

	if err := property.Load(r); err != nil {
		return nil, err
	}

	return property, nil
}

func newCastProperty(id CastPropertyId, name CastPropertyName, size uint32) (ICastProperty, error) {
	switch id {
	case CastPropertyByte:
		return &CastProperty[byte]{
			id:     id,
			name:   name,
			values: make([]byte, size),
		}, nil
	case CastPropertyShort:
		return &CastProperty[int16]{
			id:     id,
			name:   name,
			values: make([]int16, size),
		}, nil
	case CastPropertyInteger32:
		return &CastProperty[int32]{
			id:     id,
			name:   name,
			values: make([]int32, size),
		}, nil
	case CastPropertyInteger64:
		return &CastProperty[int64]{
			id:     id,
			name:   name,
			values: make([]int64, size),
		}, nil
	case CastPropertyFloat:
		return &CastProperty[float32]{
			id:     id,
			name:   name,
			values: make([]float32, size),
		}, nil
	case CastPropertyDouble:
		return &CastProperty[float64]{
			id:     id,
			name:   name,
			values: make([]float64, size),
		}, nil
	case CastPropertyString:
		return &CastProperty[string]{
			id:     id,
			name:   name,
			values: make([]string, size),
		}, nil
	case CastPropertyVector2:
		return &CastProperty[Vec2]{
			id:     id,
			name:   name,
			values: make([]Vec2, size),
		}, nil
	case CastPropertyVector3:
		return &CastProperty[Vec3]{
			id:     id,
			name:   name,
			values: make([]Vec3, size),
		}, nil

	case CastPropertyVector4:
		return &CastProperty[Vec4]{
			id:     id,
			name:   name,
			values: make([]Vec4, size),
		}, nil
	default:
		return nil, fmt.Errorf("invalid property id: %#x %v", id, id)
	}
}

type CastPropertyValueType interface {
	byte | int16 | int32 | int64 | float32 | float64 | string | Vec2 | Vec3 | Vec4
}

type CastProperty[T CastPropertyValueType] struct {
	id     CastPropertyId
	name   CastPropertyName
	values []T
}

func (p *CastProperty[T]) Name() CastPropertyName {
	return p.name
}

func (p *CastProperty[T]) Identifier() CastPropertyId {
	return p.id
}

func (p *CastProperty[T]) Load(r io.Reader) error {
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

func (p *CastProperty[T]) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, CastPropertyHeader{
		Identifier:  p.id,
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
		if _, err := w.Write([]byte(vs[0])); err != nil {
			return err
		}

		if _, err := w.Write([]byte{0x00}); err != nil {
			return err
		}
	default:
		if err := binary.Write(w, binary.LittleEndian, p.values); err != nil {
			return err
		}
	}

	return nil
}

func (p *CastProperty[T]) Length() int {
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

func (p *CastProperty[T]) ValueCount() int {
	return len(p.values)
}

type Vec2 struct {
	X, Y float32
}

type Vec3 struct {
	X, Y, Z float32
}

type Vec4 struct {
	X, Y, Z, W float32
}

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

func nextHash() uint64 {
	hash := castHashBase
	castHashBase++
	return hash
}

func createProperty[T CastPropertyValueType](node *CastNode, name CastPropertyName, id CastPropertyId, values ...T) {
	property, _ := node.CreateProperty(name, id)
	p := property.(*CastProperty[T])
	p.values = append(p.values, values...)
}

// TODO
func getPropertyValues[T CastPropertyValueType](node *CastNode, name CastPropertyName) []T {
	property, ok := node.GetProperty(name)
	if !ok {
		return []T{}
	}

	p, ok := property.(*CastProperty[T])
	if !ok {
		return []T{}
	}

	return p.values
}
