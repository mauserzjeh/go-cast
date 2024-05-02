package cast

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

const (
	castMagic uint32 = 0x74736163
)

var (
	castHashBase uint64 = 0x534E495752545250
)

type CastFile struct {
	RootNodes []ICastNode
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
		RootNodes: make([]ICastNode, header.RootNodes),
	}

	for i := range castFile.RootNodes {
		castFile.RootNodes[i] = &CastNode{}
		if err := castFile.RootNodes[i].Load(f); err != nil {
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
	CastIdRoot              CastId = 0x746F6F72
	CastIdModel             CastId = 0x6C646F6D
	CastIdMesh              CastId = 0x6873656D
	CastIdBlendShape        CastId = 0x68736C62
	CastIdSkeleton          CastId = 0x6C656B73
	CastIdBone              CastId = 0x656E6F62
	CastIdIKHandle          CastId = 0x64686B69
	CastIdConstraint        CastId = 0x74736E63
	CastIdAnimation         CastId = 0x6D696E61
	CastIdCurve             CastId = 0x76727563
	CastIdNotificationTrack CastId = 0x6669746E
	CastIdMaterial          CastId = 0x6C74616D
	CastIdFile              CastId = 0x656C6966
	CastIdInstance          CastId = 0x74736E69
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
	ChildByHash(hash uint64) ICastNode
}

type CastNode struct {
	Id         CastId
	NodeHash   uint64
	Properties map[string]ICastProperty
	ChildNodes []ICastNode
	ParentNode ICastNode
}

type CastNodes interface {
	CastNodeModel | CastNodeMesh | CastNodeBlendShape | CastNodeSkeleton | CastNodeBone | CastNodeIKHandle | CastNodeConstraint | CastNodeMaterial | CastNodeFile | CastNodeAnimation | CastNodeCurve | CastNodeNotificationTrack | CastNodeInstance
}

func NewCastNode[T CastNodes]() *T {
	var (
		def T
		id  CastId
	)

	switch any(def).(type) {
	case CastNodeModel:
		id = CastIdModel
	case CastNodeMesh:
		id = CastIdMesh
	case CastNodeBlendShape:
		id = CastIdBlendShape
	case CastNodeSkeleton:
		id = CastIdSkeleton
	case CastNodeBone:
		id = CastIdBone
	case CastNodeIKHandle:
		id = CastIdIKHandle
	case CastNodeConstraint:
		id = CastIdConstraint
	case CastNodeMaterial:
		id = CastIdMaterial
	case CastNodeFile:
		id = CastIdFile
	case CastNodeAnimation:
		id = CastIdAnimation
	case CastNodeCurve:
		id = CastIdCurve
	case CastNodeNotificationTrack:
		id = CastIdNotificationTrack
	case CastNodeInstance:
		id = CastIdInstance
	default:
		id = 0
	}

	return &T{
		CastNode: CastNode{
			Id:         id,
			NodeHash:   nextHash(),
			Properties: map[string]ICastProperty{},
			ChildNodes: []ICastNode{},
			ParentNode: nil,
		},
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
		n.Properties = make(map[string]ICastProperty)
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

func (n *CastNode) GetProperty(name string) (ICastProperty, bool) {
	property, ok := n.Properties[name]
	return property, ok
}

func (n *CastNode) CreateProperty(name string, id CastPropertyId) (ICastProperty, error) {
	property, err := newCastProperty(id, name, 0)
	if err != nil {
		return nil, err
	}

	if n.Properties == nil {
		n.Properties = make(map[string]ICastProperty)
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

func NewCastNodeModel() *CastNodeModel {
	return NewCastNode[CastNodeModel]()
}

func (n *CastNodeModel) Name() string {
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
	skeletons := n.ChildrenOfType(CastIdSkeleton)
	if len(skeletons) > 0 {
		return (skeletons[0]).(*CastNodeSkeleton)
	}

	return nil
}

func (n *CastNodeModel) CreateSkeleton() *CastNodeSkeleton {
	return (n.CreateChild(NewCastNodeSkeleton())).(*CastNodeSkeleton)
}

func (n *CastNodeModel) Meshes() []*CastNodeMesh {
	ms := n.ChildrenOfType(CastIdMesh)
	meshes := make([]*CastNodeMesh, len(ms))

	for i := range meshes {
		meshes[i] = (ms[i]).(*CastNodeMesh)
	}

	return meshes
}

func (n *CastNodeModel) CreateMesh() *CastNodeMesh {
	return (n.CreateChild(NewCastNodeMesh())).(*CastNodeMesh)
}

func (n *CastNodeModel) Materials() []*CastNodeMaterial {
	ms := n.ChildrenOfType(CastIdMaterial)
	materials := make([]*CastNodeMaterial, len(ms))

	for i := range materials {
		materials[i] = (ms[i]).(*CastNodeMaterial)
	}

	return materials
}

func (n *CastNodeModel) CreateMaterial() *CastNodeMaterial {
	return (n.CreateChild(NewCastNodeMaterial())).(*CastNodeMaterial)
}

func (n *CastNodeModel) BlendShapes() []*CastNodeBlendShape {
	bs := n.ChildrenOfType(CastIdBlendShape)
	blendShapes := make([]*CastNodeBlendShape, len(bs))

	for i := range blendShapes {
		blendShapes[i] = (bs[i]).(*CastNodeBlendShape)
	}

	return blendShapes
}

func (n *CastNodeModel) CreateBlendShape() *CastNodeBlendShape {
	return (n.CreateChild(NewCastNodeBlendShape())).(*CastNodeBlendShape)
}

type CastNodeMesh struct{ CastNode }

func NewCastNodeMesh() *CastNodeMesh {
	return NewCastNode[CastNodeMesh]()
}

func (n *CastNodeMesh) Name() string {
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
func (n *CastNodeMesh) FaceBuffer() []uint32 {
	return getPropertyValues[uint32](&n.CastNode, CastPropertyNameFaceBuffer)
}

func (n *CastNodeMesh) SetFaceBuffer(values []uint32) {
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

func (n *CastNodeMesh) VertexColorBuffer() []uint32 {
	return getPropertyValues[uint32](&n.CastNode, CastPropertyNameVertexColorBuffer)
}

func (n *CastNodeMesh) SetVertexColorBuffer(values []uint32) {
	createProperty(&n.CastNode, CastPropertyNameVertexColorBuffer, CastPropertyInteger32, values...)
}

func (n *CastNodeMesh) VertexUVLayerBuffer(index int) []Vec2 {
	return getPropertyValues[Vec2](&n.CastNode, fmt.Sprintf(CastPropertyNameVertexUVBuffer, index))
}

func (n *CastNodeMesh) SetVertexUVLayerBuffer(index int, values []Vec2) {
	createProperty(&n.CastNode, fmt.Sprintf(CastPropertyNameVertexUVBuffer, index), CastPropertyVector2, values...)
}

// TODO
func (n *CastNodeMesh) VertexWeightBoneBuffer() []uint32 {
	return getPropertyValues[uint32](&n.CastNode, "wb")
}

func (n *CastNodeMesh) SetVertexWeightBoneBuffer(values []uint32) {
	createProperty(&n.CastNode, "wb", CastPropertyInteger32, values...)
}

func (n *CastNodeMesh) VertexWeightValueBuffer() []float32 {
	return getPropertyValues[float32](&n.CastNode, CastPropertyNameVertexWeightBoneBuffer)
}

func (n *CastNodeMesh) SetVertexWeightValueBuffer(values []float32) {
	createProperty(&n.CastNode, CastPropertyNameVertexWeightBoneBuffer, CastPropertyFloat, values...)
}

func (n *CastNodeMesh) Material() *CastNodeMaterial {
	materialHashes := getPropertyValues[uint64](&n.CastNode, CastPropertyNameMaterial)
	if len(materialHashes) == 0 {
		return nil
	}

	m := n.ChildByHash(materialHashes[0])
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
	createProperty(&n.CastNode, CastPropertyNameMaterial, CastPropertyInteger64, hash)
}

type CastNodeBlendShape struct{ CastNode }

func NewCastNodeBlendShape() *CastNodeBlendShape {
	return NewCastNode[CastNodeBlendShape]()
}

func (n *CastNodeBlendShape) Name() string {
	property, ok := n.GetProperty(CastPropertyNameName)
	if !ok {
		return ""
	}

	return property.Name()
}

func (n *CastNodeBlendShape) SetName(name string) {
	createProperty(&n.CastNode, CastPropertyNameName, CastPropertyString, name)
}

func (n *CastNodeBlendShape) BaseShape() *CastNodeMesh {
	meshHashes := getPropertyValues[uint64](&n.CastNode, CastPropertyNameBaseShape)
	if len(meshHashes) == 0 {
		return nil
	}

	b := n.ChildByHash(meshHashes[0])
	if b == nil {
		return nil
	}

	baseShape, ok := b.(*CastNodeMesh)
	if !ok {
		return nil
	}

	return baseShape
}

func (n *CastNodeBlendShape) SetBaseShape(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNameBaseShape, CastPropertyInteger64, hash)
}

func (n *CastNodeBlendShape) TargetShapes() []*CastNodeMesh {
	meshHashes := getPropertyValues[uint64](&n.CastNode, CastPropertyNameTargetShape)
	if len(meshHashes) == 0 {
		return []*CastNodeMesh{}
	}

	targetShapes := make([]*CastNodeMesh, 0)
	for _, hash := range meshHashes {
		t := n.ChildByHash(hash)
		if t == nil {
			continue
		}

		targetShape, ok := t.(*CastNodeMesh)
		if !ok {
			continue
		}

		targetShapes = append(targetShapes, targetShape)
	}

	return targetShapes
}

func (n *CastNodeBlendShape) SetTargetShapes(hashes []uint64) {
	createProperty(&n.CastNode, CastPropertyNameTargetShape, CastPropertyInteger64, hashes...)
}

func (n *CastNodeBlendShape) TargetWeightScales() []float32 {
	return getPropertyValues[float32](&n.CastNode, CastPropertyNameTargetWeightScale)
}

func (n *CastNodeBlendShape) SetTargetWeightScales(values []float32) {
	createProperty(&n.CastNode, CastPropertyNameTargetWeightScale, CastPropertyFloat, values...)
}

type CastNodeSkeleton struct{ CastNode }

func NewCastNodeSkeleton() *CastNodeSkeleton {
	return NewCastNode[CastNodeSkeleton]()
}

func (n *CastNodeSkeleton) Bones() []*CastNodeBone {
	bs := n.ChildrenOfType(CastIdBone)
	bones := make([]*CastNodeBone, len(bs))

	for i := range bones {
		bones[i] = (bs[i]).(*CastNodeBone)
	}

	return bones
}

func (n *CastNodeSkeleton) CreateBone() *CastNodeBone {
	return (n.CreateChild(NewCastNodeBone())).(*CastNodeBone)
}

func (n *CastNodeSkeleton) IKHandles() []*CastNodeIKHandle {
	iks := n.ChildrenOfType(CastIdIKHandle)
	ikHandles := make([]*CastNodeIKHandle, len(iks))

	for i := range ikHandles {
		ikHandles[i] = (iks[i]).(*CastNodeIKHandle)
	}

	return ikHandles
}

func (n *CastNodeSkeleton) CreateIKHandle() *CastNodeIKHandle {
	return (n.CreateChild(NewCastNodeIKHandle())).(*CastNodeIKHandle)
}

func (n *CastNodeSkeleton) Constraints() []*CastNodeConstraint {
	cs := n.ChildrenOfType(CastIdConstraint)
	constraints := make([]*CastNodeConstraint, len(cs))

	for i := range constraints {
		constraints[i] = (cs[i]).(*CastNodeConstraint)
	}

	return constraints
}

func (n *CastNodeSkeleton) CreateConstraint() *CastNodeConstraint {
	return (n.CreateChild(NewCastNodeConstraint())).(*CastNodeConstraint)
}

type CastNodeBone struct{ CastNode }

func NewCastNodeBone() *CastNodeBone {
	return NewCastNode[CastNodeBone]()
}

func (n *CastNodeBone) Name() string {
	property, ok := n.GetProperty(CastPropertyNameName)
	if !ok {
		return ""
	}

	return property.Name()
}

func (n *CastNodeBone) SetName(name string) {
	createProperty(&n.CastNode, CastPropertyNameName, CastPropertyString, name)
}

func (n *CastNodeBone) ParentIndex() int32 {
	values := getPropertyValues[uint32](&n.CastNode, CastPropertyNameParentIndex)
	if len(values) == 0 {
		return -1
	}
	return int32((values[0] & 0xFFFFFFFF) ^ 0x80000000 - 0x80000000)
}

func (n *CastNodeBone) SetParentIndex(index int32) {
	idx := uint32(index)
	if index < 0 {
		idx = uint32(index) + uint32(math.Pow(2, 32))
	}
	createProperty(&n.CastNode, CastPropertyNameParentIndex, CastPropertyInteger32, idx)
}

func (n *CastNodeBone) SegmentScaleCompensate() bool {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameSegmentScaleCompensate)
	if len(values) == 0 {
		return true
	}

	return values[0] >= 1
}

func (n *CastNodeBone) SetSegmentScaleCompensate(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.CastNode, CastPropertyNameSegmentScaleCompensate, CastPropertyByte, value)
}

func (n *CastNodeBone) LocalPosition() Vec3 {
	values := getPropertyValues[Vec3](&n.CastNode, CastPropertyNameLocalPosition)
	if len(values) == 0 {
		return Vec3{}
	}

	return values[0]
}

func (n *CastNodeBone) SetLocalPosition(position Vec3) {
	createProperty(&n.CastNode, CastPropertyNameLocalPosition, CastPropertyVector3, position)
}

func (n *CastNodeBone) LocalRotation() Vec4 {
	values := getPropertyValues[Vec4](&n.CastNode, CastPropertyNameLocalRotation)
	if len(values) == 0 {
		return Vec4{}
	}

	return values[0]
}
func (n *CastNodeBone) SetLocalRotation(rotation Vec4) {
	createProperty(&n.CastNode, CastPropertyNameLocalRotation, CastPropertyVector4, rotation)
}

func (n *CastNodeBone) WorldPosition() Vec3 {
	values := getPropertyValues[Vec3](&n.CastNode, CastPropertyNameWorldPosition)
	if len(values) == 0 {
		return Vec3{}
	}

	return values[0]
}

func (n *CastNodeBone) SetWorldPosition(position Vec3) {
	createProperty(&n.CastNode, CastPropertyNameWorldPosition, CastPropertyVector3, position)
}

func (n *CastNodeBone) WorldRotation() Vec4 {
	values := getPropertyValues[Vec4](&n.CastNode, CastPropertyNameWorldRotation)
	if len(values) == 0 {
		return Vec4{}
	}

	return values[0]
}
func (n *CastNodeBone) SetWorldRotation(rotation Vec4) {
	createProperty(&n.CastNode, CastPropertyNameWorldRotation, CastPropertyVector4, rotation)
}

func (n *CastNodeBone) Scale() Vec3 {
	values := getPropertyValues[Vec3](&n.CastNode, CastPropertyNameScale)
	if len(values) == 0 {
		return Vec3{}
	}

	return values[0]
}

func (n *CastNodeBone) SetScale(scale Vec3) {
	createProperty(&n.CastNode, CastPropertyNameScale, CastPropertyVector3, scale)
}

type CastNodeIKHandle struct{ CastNode }

func NewCastNodeIKHandle() *CastNodeIKHandle {
	return NewCastNode[CastNodeIKHandle]()
}

func (n *CastNodeIKHandle) Name() string {
	property, ok := n.GetProperty(CastPropertyNameName)
	if !ok {
		return ""
	}

	return property.Name()
}

func (n *CastNodeIKHandle) SetName(name string) {
	createProperty(&n.CastNode, CastPropertyNameName, CastPropertyString, name)
}

func (n *CastNodeIKHandle) StartBone() *CastNodeBone {
	values := getPropertyValues[uint64](&n.CastNode, CastPropertyNameStartBone)
	if len(values) == 0 {
		return nil
	}

	if n.ParentNode == nil {
		return nil
	}

	sb := n.ParentNode.ChildByHash(values[0])
	if sb == nil {
		return nil
	}

	startBone, ok := sb.(*CastNodeBone)
	if !ok {
		return nil
	}

	return startBone
}

func (n *CastNodeIKHandle) SetStartBone(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNameStartBone, CastPropertyInteger64, hash)
}

func (n *CastNodeIKHandle) EndBone() *CastNodeBone {
	values := getPropertyValues[uint64](&n.CastNode, CastPropertyNameEndBone)
	if len(values) == 0 {
		return nil
	}

	if n.ParentNode == nil {
		return nil
	}

	eb := n.ParentNode.ChildByHash(values[0])
	if eb == nil {
		return nil
	}

	endBone, ok := eb.(*CastNodeBone)
	if !ok {
		return nil
	}

	return endBone
}

func (n *CastNodeIKHandle) SetEndBone(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNameEndBone, CastPropertyInteger64, hash)
}

func (n *CastNodeIKHandle) TargetBone() *CastNodeBone {
	values := getPropertyValues[uint64](&n.CastNode, CastPropertyNameTargetBone)
	if len(values) == 0 {
		return nil
	}

	if n.ParentNode == nil {
		return nil
	}

	tb := n.ParentNode.ChildByHash(values[0])
	if tb == nil {
		return nil
	}

	targetBone, ok := tb.(*CastNodeBone)
	if !ok {
		return nil
	}

	return targetBone
}

func (n *CastNodeIKHandle) SetTargetBone(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNameTargetBone, CastPropertyInteger64, hash)
}

func (n *CastNodeIKHandle) PoleVectorBone() *CastNodeBone {
	values := getPropertyValues[uint64](&n.CastNode, CastPropertyNamePoleVectorBone)
	if len(values) == 0 {
		return nil
	}

	if n.ParentNode == nil {
		return nil
	}

	pv := n.ParentNode.ChildByHash(values[0])
	if pv == nil {
		return nil
	}

	poleVectorPone, ok := pv.(*CastNodeBone)
	if !ok {
		return nil
	}

	return poleVectorPone
}

func (n *CastNodeIKHandle) SetPoleVectorBone(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNamePoleVectorBone, CastPropertyInteger64, hash)
}

func (n *CastNodeIKHandle) PoleBone() *CastNodeBone {
	values := getPropertyValues[uint64](&n.CastNode, CastPropertyNamePoleBone)
	if len(values) == 0 {
		return nil
	}

	if n.ParentNode == nil {
		return nil
	}

	pb := n.ParentNode.ChildByHash(values[0])
	if pb == nil {
		return nil
	}

	poleBone, ok := pb.(*CastNodeBone)
	if !ok {
		return nil
	}

	return poleBone
}

func (n *CastNodeIKHandle) SetPoleBone(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNamePoleBone, CastPropertyInteger64, hash)
}

func (n *CastNodeIKHandle) UseTargetRotation() bool {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameTargetRotation)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

func (n *CastNodeIKHandle) SetUseTargetRotation(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.CastNode, CastPropertyNameTargetRotation, CastPropertyByte, value)
}

type CastNodeConstraint struct{ CastNode }

func NewCastNodeConstraint() *CastNodeConstraint {
	return NewCastNode[CastNodeConstraint]()
}

func (n *CastNodeConstraint) Name() string {
	property, ok := n.GetProperty(CastPropertyNameName)
	if !ok {
		return ""
	}

	return property.Name()
}

func (n *CastNodeConstraint) SetName(name string) {
	createProperty(&n.CastNode, CastPropertyNameName, CastPropertyString, name)
}

func (n *CastNodeConstraint) ConstraintType() string {
	values := getPropertyValues[string](&n.CastNode, CastPropertyNameConstraintType)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (n *CastNodeConstraint) SetConstraintType(constraintType string) {
	createProperty(&n.CastNode, CastPropertyNameConstraintType, CastPropertyString, constraintType)
}

func (n *CastNodeConstraint) ConstraintBone() *CastNodeBone {
	values := getPropertyValues[uint64](&n.CastNode, CastPropertyNameConstraintBone)
	if len(values) == 0 {
		return nil
	}

	if n.ParentNode == nil {
		return nil
	}

	cb := n.ParentNode.ChildByHash(values[0])
	if cb == nil {
		return nil
	}

	constraintBone, ok := cb.(*CastNodeBone)
	if !ok {
		return nil
	}

	return constraintBone
}

func (n *CastNodeConstraint) SetConstraintBone(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNameConstraintBone, CastPropertyInteger64, hash)
}

func (n *CastNodeConstraint) TargetBone() *CastNodeBone {
	values := getPropertyValues[uint64](&n.CastNode, CastPropertyNameTargetBone)
	if len(values) == 0 {
		return nil
	}

	if n.ParentNode == nil {
		return nil
	}

	tb := n.ParentNode.ChildByHash(values[0])
	if tb == nil {
		return nil
	}

	targetBone, ok := tb.(*CastNodeBone)
	if !ok {
		return nil
	}

	return targetBone
}

func (n *CastNodeConstraint) SetTargetBone(hash uint64) {
	createProperty(&n.CastNode, CastPropertyNameTargetBone, CastPropertyInteger64, hash)
}

func (n *CastNodeConstraint) MaintainOffset() bool {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameMaintainOffset)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

func (n *CastNodeConstraint) SetMaintainOffset(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.CastNode, CastPropertyNameMaintainOffset, CastPropertyByte, value)
}

func (n *CastNodeConstraint) SkipX() bool {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameSkipX)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

func (n *CastNodeConstraint) SetSkipX(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.CastNode, CastPropertyNameSkipX, CastPropertyByte, value)
}

func (n *CastNodeConstraint) SkipY() bool {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameSkipY)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

func (n *CastNodeConstraint) SetSkipY(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.CastNode, CastPropertyNameSkipY, CastPropertyByte, value)
}

func (n *CastNodeConstraint) SkipZ() bool {
	values := getPropertyValues[byte](&n.CastNode, CastPropertyNameSkipZ)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

func (n *CastNodeConstraint) SetSkipZ(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.CastNode, CastPropertyNameSkipZ, CastPropertyByte, value)
}

type CastNodeMaterial struct{ CastNode }

func NewCastNodeMaterial() *CastNodeMaterial {
	return NewCastNode[CastNodeMaterial]()
}

func (n *CastNodeMaterial) Name() string {
	property, ok := n.GetProperty(CastPropertyNameName)
	if !ok {
		return ""
	}

	return property.Name()
}

func (n *CastNodeMaterial) SetName(name string) {
	createProperty(&n.CastNode, CastPropertyNameName, CastPropertyString, name)
}

func (n *CastNodeMaterial) Type() string {
	values := getPropertyValues[string](&n.CastNode, CastPropertyNameType)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (n *CastNodeMaterial) SetType(materialType string) {
	createProperty(&n.CastNode, CastPropertyNameType, CastPropertyString, materialType)
}

func (n *CastNodeMaterial) Slots() map[string]*CastNodeFile {
	slots := make(map[string]*CastNodeFile, 0)
	for slot, property := range n.Properties {
		if slot != CastPropertyNameName && slot != CastPropertyNameType {
			p, ok := property.(*CastProperty[uint64])
			if !ok {
				continue
			}

			if len(p.values) == 0 {
				continue
			}

			nodeFile := n.ChildByHash(p.values[0])
			if nodeFile == nil {
				continue
			}

			nf, ok := nodeFile.(*CastNodeFile)
			if !ok {
				continue
			}

			slots[slot] = nf
		}
	}

	return slots
}

func (n *CastNodeMaterial) CreateFile() *CastNodeFile {
	return (n.CreateChild(NewCastNodeFile())).(*CastNodeFile)
}

type CastNodeFile struct{ CastNode }

func NewCastNodeFile() *CastNodeFile {
	return NewCastNode[CastNodeFile]()
}

func (n *CastNodeFile) Path() string {
	values := getPropertyValues[string](&n.CastNode, CastPropertyNamePath)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (n *CastNodeFile) SetPath(path string) {
	createProperty(&n.CastNode, CastPropertyNamePath, CastPropertyString, path)
}

type CastNodeAnimation struct{ CastNode }

func NewCastNodeAnimation() *CastNodeAnimation {
	return NewCastNode[CastNodeAnimation]()
}

type CastNodeCurve struct{ CastNode }

func NewCastNodeCurve() *CastNodeCurve {
	return NewCastNode[CastNodeCurve]()
}

type CastNodeNotificationTrack struct{ CastNode }

func NewCastNodeNotificationTrack() *CastNodeNotificationTrack {
	return NewCastNode[CastNodeNotificationTrack]()
}

type CastNodeInstance struct{ CastNode }

func NewCastNodeInstance() *CastNodeInstance {
	return NewCastNode[CastNodeInstance]()
}

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

const (
	CastPropertyNameName                   = "n"
	CastPropertyNameVertexPositionBuffer   = "vp"
	CastPropertyNameVertexNormalBuffer     = "vn"
	CastPropertyNameVertexTangentBuffer    = "vt"
	CastPropertyNameVertexColorBuffer      = "vc"
	CastPropertyNameVertexUVBuffer         = "u%d"
	CastPropertyNameVertexWeightBoneBuffer = "wv"
	CastPropertyNameFaceBuffer             = "f"
	CastPropertyNameUVLayerCount           = "ul"
	CastPropertyNameMaximumWeightInfluence = "mi"
	CastPropertyNameSkinningMethod         = "sm"
	CastPropertyNameMaterial               = "m"
	CastPropertyNameBaseShape              = "b"
	CastPropertyNameTargetShape            = "t"
	CastPropertyNameTargetWeightScale      = "ts"
	CastPropertyNameParentIndex            = "p"
	CastPropertyNameSegmentScaleCompensate = "ssc"
	CastPropertyNameLocalPosition          = "lp"
	CastPropertyNameLocalRotation          = "lr"
	CastPropertyNameWorldPosition          = "wp"
	CastPropertyNameWorldRotation          = "wr"
	CastPropertyNameScale                  = "s"
	CastPropertyNameStartBone              = "sb"
	CastPropertyNameEndBone                = "eb"
	CastPropertyNameTargetBone             = "tb"
	CastPropertyNamePoleVectorBone         = "pv"
	CastPropertyNamePoleBone               = "pb"
	CastPropertyNameTargetRotation         = "tr"
	CastPropertyNameConstraintType         = "ct"
	CastPropertyNameConstraintBone         = "cb"
	CastPropertyNameMaintainOffset         = "mo"
	CastPropertyNameSkipX                  = "sx"
	CastPropertyNameSkipY                  = "sy"
	CastPropertyNameSkipZ                  = "sz"
	CastPropertyNameType                   = "t"
	CastPropertyNamePath                   = "p"
)

type CastPropertyHeader struct {
	Identifier  CastPropertyId
	NameSize    uint16
	ArrayLength uint32
}

type ICastProperty interface {
	Name() string
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

	property, err := newCastProperty(header.Identifier, string(name), header.ArrayLength)
	if err != nil {
		return nil, err
	}

	if err := property.Load(r); err != nil {
		return nil, err
	}

	return property, nil
}

func newCastProperty(id CastPropertyId, name string, size uint32) (ICastProperty, error) {
	switch id {
	case CastPropertyByte:
		return &CastProperty[byte]{
			id:     id,
			name:   name,
			values: make([]byte, size),
		}, nil
	case CastPropertyShort:
		return &CastProperty[uint16]{
			id:     id,
			name:   name,
			values: make([]uint16, size),
		}, nil
	case CastPropertyInteger32:
		return &CastProperty[uint32]{
			id:     id,
			name:   name,
			values: make([]uint32, size),
		}, nil
	case CastPropertyInteger64:
		return &CastProperty[uint64]{
			id:     id,
			name:   name,
			values: make([]uint64, size),
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
	byte | uint16 | uint32 | uint64 | float32 | float64 | string | Vec2 | Vec3 | Vec4
}

type CastProperty[T CastPropertyValueType] struct {
	id     CastPropertyId
	name   string
	values []T
}

func (p *CastProperty[T]) Name() string {
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

func createProperty[T CastPropertyValueType](node *CastNode, name string, id CastPropertyId, values ...T) {
	property, _ := node.CreateProperty(name, id)
	p := property.(*CastProperty[T])
	p.values = append(p.values, values...)
}

// TODO
func getPropertyValues[T CastPropertyValueType](node *CastNode, name string) []T {
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
