package cast

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const (
	castMagic uint32 = 0x74736163
)

var (
	castHashBase uint64 = 0x534E495752545250
)

// CastFile holds data of a cast file
type CastFile struct {
	flags     uint32
	version   uint32
	rootNodes []*CastNodeRoot
}

// New creates a new [CastFile]
func New() *CastFile {
	return &CastFile{
		flags:     0,
		version:   0x1,
		rootNodes: make([]*CastNodeRoot, 0),
	}
}

// Load loads a [CastFile] from the given [io.Reader]
func Load(r io.Reader) (*CastFile, error) {
	var header castHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Magic != castMagic {
		return nil, fmt.Errorf("invalid cast file magic: %#x", header.Magic)
	}

	castFile := &CastFile{
		flags:     header.Flags,
		version:   header.Version,
		rootNodes: make([]*CastNodeRoot, header.RootNodes),
	}

	for i := range castFile.rootNodes {
		castFile.rootNodes[i] = &CastNodeRoot{}
		if err := castFile.rootNodes[i].Load(r); err != nil {
			return nil, err
		}
	}
	return castFile, nil
}

// Flags returns the flags
func (n *CastFile) Flags() uint32 {
	return n.flags
}

// SetFlags sets the flags
func (n *CastFile) SetFlags(flags uint32) *CastFile {
	n.flags = flags
	return n
}

// Version returns the version
func (n *CastFile) Version() uint32 {
	return n.version
}

// SetVersion sets the version
func (n *CastFile) SetVersion(version uint32) *CastFile {
	n.version = version
	return n
}

// Roots returns the root nodes
func (n *CastFile) Roots() []*CastNodeRoot {
	return n.rootNodes
}

// CreateRoot creates a root node
func (n *CastFile) CreateRoot() *CastNodeRoot {
	root := NewCastNodeRoot()
	n.rootNodes = append(n.rootNodes, root)
	return root
}

// Write writes the file to the given [io.Writer]
func (n *CastFile) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, castHeader{
		Magic:     castMagic,
		Version:   n.version,
		RootNodes: uint32(len(n.rootNodes)),
		Flags:     n.flags,
	}); err != nil {
		return err
	}

	for _, rootNode := range n.rootNodes {
		if err := rootNode.Write(w); err != nil {
			return err
		}
	}

	return nil
}

// castHeader holds header data of the cast file
type castHeader struct {
	Magic     uint32
	Version   uint32
	RootNodes uint32
	Flags     uint32
}

// CastId tyoe alias
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

// castNodeHeader hold header data of a node
type castNodeHeader struct {
	Identifier    CastId
	NodeSize      uint32
	NodeHash      uint64
	PropertyCount uint32
	ChildCount    uint32
}

// iCastNode is the interface for thhe node
type iCastNode interface {
	Identifier() CastId
	Hash() uint64
	Load(r io.Reader) error
	SetParentNode(node iCastNode)
	GetParentNode() iCastNode
	GetCastNode() *castNode
	Lenght() int
	Write(w io.Writer) error
	ChildByHash(hash uint64) iCastNode
}

// castNode holds data of a node
type castNode struct {
	id         CastId
	nodeHash   uint64
	properties map[string]iCastProperty
	childNodes []iCastNode
	parentNode iCastNode
}

type CastNodes interface {
	CastNodeRoot | CastNodeModel | CastNodeMesh | CastNodeBlendShape | CastNodeSkeleton | CastNodeBone | CastNodeIKHandle | CastNodeConstraint | CastNodeMaterial | CastNodeFile | CastNodeAnimation | CastNodeCurve | CastNodeNotificationTrack | CastNodeInstance
}

// NewCastNode creates a new node
func NewCastNode[T CastNodes]() *T {
	var (
		def T
		id  CastId
	)

	switch any(def).(type) {
	case CastNodeRoot:
		id = CastIdRoot
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
		castNode: castNode{
			id:         id,
			nodeHash:   nextHash(),
			properties: map[string]iCastProperty{},
			childNodes: []iCastNode{},
			parentNode: nil,
		},
	}
}

// Identifier returns the id
func (n *castNode) Identifier() CastId {
	return n.id
}

// Hash returns the hash
func (n *castNode) Hash() uint64 {
	return n.nodeHash
}

// Load loads a node from the given [io.Reader]
func (n *castNode) Load(r io.Reader) error {
	var header castNodeHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return err
	}

	if n.properties == nil {
		n.properties = make(map[string]iCastProperty)
	}

	for range header.PropertyCount {
		property, err := loadCastProperty(r)
		if err != nil {
			return err
		}

		n.properties[property.Name()] = property
	}

	n.childNodes = make([]iCastNode, header.ChildCount)
	for i := range n.childNodes {
		n.childNodes[i] = &castNode{}
		if err := n.childNodes[i].Load(r); err != nil {
			return err
		}
		n.childNodes[i].SetParentNode(n)
	}

	return nil
}

// Write writes the node to the given [io.Writer]
func (n *castNode) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, castNodeHeader{
		Identifier:    n.id,
		NodeSize:      uint32(n.Lenght()),
		NodeHash:      n.nodeHash,
		PropertyCount: uint32(len(n.properties)),
		ChildCount:    uint32(len(n.childNodes)),
	}); err != nil {
		return err
	}

	for _, p := range n.properties {
		if err := p.Write(w); err != nil {
			return err
		}
	}

	for _, c := range n.childNodes {
		if err := c.Write(w); err != nil {
			return err
		}
	}

	return nil
}

// SetParentNode sets the parent node
func (n *castNode) SetParentNode(node iCastNode) {
	n.parentNode = node
}

// GetParentNode returns the parent node
func (n *castNode) GetParentNode() iCastNode {
	return n.parentNode
}

// GetCastNode returns the node
func (n *castNode) GetCastNode() *castNode {
	return n
}

// Lenght returns the size of the node
func (n *castNode) Lenght() int {
	l := 0x18

	for _, p := range n.properties {
		l += p.Length()
	}

	for _, c := range n.childNodes {
		l += c.Lenght()
	}
	return l
}

// GetProperty returns the property with the given name
func (n *castNode) GetProperty(name string) (iCastProperty, bool) {
	property, ok := n.properties[name]
	return property, ok
}

// CreateProperty creates a new property with the given name and type
func (n *castNode) CreateProperty(name string, id CastPropertyId) (iCastProperty, error) {
	property, err := newCastProperty(id, name, 0)
	if err != nil {
		return nil, err
	}

	if n.properties == nil {
		n.properties = make(map[string]iCastProperty)
	}

	n.properties[name] = property
	return property, nil
}

// ChildrenOfType returns childnodes with the given type
func (n *castNode) ChildrenOfType(id CastId) []iCastNode {
	nodes := make([]iCastNode, 0)
	for _, c := range n.childNodes {
		if c.Identifier() == id {
			nodes = append(nodes, c)
		}
	}

	return nodes
}

// ChildByHash returns a childnode with the given hash
func (n *castNode) ChildByHash(hash uint64) iCastNode {
	nodes := make([]iCastNode, 0)
	for _, c := range n.childNodes {
		if c.Hash() == hash {
			nodes = append(nodes, c)
		}
	}

	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

// CreateChild creates a new childnode
func (n *castNode) CreateChild(child iCastNode) iCastNode {
	child.SetParentNode(n)
	n.childNodes = append(n.childNodes, child)
	return child
}

// GetProperties returns the properties
func (n *castNode) GetProperties() map[string]iCastProperty {
	return n.properties
}

// GetChildNodes returns the child nodes
func (n *castNode) GetChildNodes() []iCastNode {
	return n.childNodes
}

type CastNodeRoot struct{ castNode }

// NewCastNodeRoot creates a new root node
func NewCastNodeRoot() *CastNodeRoot {
	return NewCastNode[CastNodeRoot]()
}

// CreateModel creates a new model node
func (n *CastNodeRoot) CreateModel() *CastNodeModel {
	return (n.CreateChild(NewCastNodeModel())).(*CastNodeModel)
}

// CreateAnimation creates a new animation node
func (n *CastNodeRoot) CreateAnimation() *CastNodeAnimation {
	return (n.CreateChild(NewCastNodeAnimation())).(*CastNodeAnimation)
}

// CreateInstance creates a new instance node
func (n *CastNodeRoot) CreateInstance() *CastNodeInstance {
	return (n.CreateChild(NewCastNodeInstance())).(*CastNodeInstance)
}

type CastNodeModel struct{ castNode }

// NewCastNodeModel creates a new model node
func NewCastNodeModel() *CastNodeModel {
	return NewCastNode[CastNodeModel]()
}

// Name returns the name property
func (n *CastNodeModel) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeModel) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// Skeleton returns the skeleton node
func (n *CastNodeModel) Skeleton() *CastNodeSkeleton {
	return getChildOfType[CastNodeSkeleton](&n.castNode, CastIdSkeleton)
}

// CreateSkeleton creates a new skeleton node
func (n *CastNodeModel) CreateSkeleton() *CastNodeSkeleton {
	return (n.CreateChild(NewCastNodeSkeleton())).(*CastNodeSkeleton)
}

// Meshes returns the mesh nodes
func (n *CastNodeModel) Meshes() []*CastNodeMesh {
	return getChildrenOfType[CastNodeMesh](&n.castNode, CastIdMesh)
}

// CreateMesh creates a new mesh node
func (n *CastNodeModel) CreateMesh() *CastNodeMesh {
	return (n.CreateChild(NewCastNodeMesh())).(*CastNodeMesh)
}

// Materials returns the material nodes
func (n *CastNodeModel) Materials() []*CastNodeMaterial {
	return getChildrenOfType[CastNodeMaterial](&n.castNode, CastIdMaterial)
}

// CreateMaterial creates a new material node
func (n *CastNodeModel) CreateMaterial() *CastNodeMaterial {
	return (n.CreateChild(NewCastNodeMaterial())).(*CastNodeMaterial)
}

// BlendShapes returns the blendshape nodes
func (n *CastNodeModel) BlendShapes() []*CastNodeBlendShape {
	return getChildrenOfType[CastNodeBlendShape](&n.castNode, CastIdBlendShape)
}

// CreateBlendShape creates a new blendshape node
func (n *CastNodeModel) CreateBlendShape() *CastNodeBlendShape {
	return (n.CreateChild(NewCastNodeBlendShape())).(*CastNodeBlendShape)
}

type CastNodeMesh struct{ castNode }

// NewCastNodeMesh creates a new mesh node
func NewCastNodeMesh() *CastNodeMesh {
	return NewCastNode[CastNodeMesh]()
}

// Name returns the name property
func (n *CastNodeMesh) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeMesh) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// VertexCount returns the amount of vertices
func (n *CastNodeMesh) VertexCount() int {
	property, ok := n.GetProperty(CastPropertyNameVertexPositionBuffer)
	if !ok {
		return 0
	}
	return property.ValueCount()
}

// FaceCount returns the amount of faces
func (n *CastNodeMesh) FaceCount() int {
	property, ok := n.GetProperty(CastPropertyNameFaceBuffer)
	if !ok {
		return 0
	}
	return property.ValueCount() / 3
}

// UVLayerCount returns the amount of UV layers
func (n *CastNodeMesh) UVLayerCount() int {
	// TODO
	return int(getPropertyValue[byte](&n.castNode, CastPropertyNameUVLayerCount))
}

// SetUVLayerCount sets the amount of UV layers
func (n *CastNodeMesh) SetUVLayerCount(count byte) {
	createProperty(&n.castNode, CastPropertyNameUVLayerCount, CastPropertyByte, count)
}

// MaximumWeightInfluence returns the maximum weight influence
func (n *CastNodeMesh) MaximumWeightInfluence() int {
	// TODO
	return int(getPropertyValue[byte](&n.castNode, CastPropertyNameMaximumWeightInfluence))
}

// SetMaximumWeightInfluence sets the maximum weight influence
func (n *CastNodeMesh) SetMaximumWeightInfluence(maximum byte) {
	createProperty(&n.castNode, CastPropertyNameMaximumWeightInfluence, CastPropertyByte, maximum)
}

// SkinningMethod returns the skinning method
func (n *CastNodeMesh) SkinningMethod() string {
	values := getPropertyValues[string](&n.castNode, CastPropertyNameSkinningMethod)
	if len(values) == 0 {
		return "linear"
	}
	return values[0]
}

// SetSkinningMethod sets the skinning method
func (n *CastNodeMesh) SetSkinningMethod(method string) {
	createProperty(&n.castNode, CastPropertyNameSkinningMethod, CastPropertyString, method)
}

// FaceBuffer returns the face buffer
func (n *CastNodeMesh) FaceBuffer() []uint32 {
	// TODO
	return getPropertyValues[uint32](&n.castNode, CastPropertyNameFaceBuffer)
}

// SetFaceBuffer sets the face buffer
func (n *CastNodeMesh) SetFaceBuffer(values []uint32) {
	createProperty(&n.castNode, CastPropertyNameFaceBuffer, CastPropertyInteger32, values...)
}

// VertexPositionBuffer returns the vertex position buffer
func (n *CastNodeMesh) VertexPositionBuffer() []Vec3 {
	return getPropertyValues[Vec3](&n.castNode, CastPropertyNameVertexPositionBuffer)
}

// SetVertexPositionBuffer sets the vertex position buffer
func (n *CastNodeMesh) SetVertexPositionBuffer(values []Vec3) {
	createProperty(&n.castNode, CastPropertyNameVertexPositionBuffer, CastPropertyVector3, values...)
}

// VertexNormalBuffer returns the vertex normal buffer
func (n *CastNodeMesh) VertexNormalBuffer() []Vec3 {
	return getPropertyValues[Vec3](&n.castNode, CastPropertyNameVertexNormalBuffer)
}

// SetVertexNormalBuffer sets the vertex normal buffer
func (n *CastNodeMesh) SetVertexNormalBuffer(values []Vec3) {
	createProperty(&n.castNode, CastPropertyNameVertexNormalBuffer, CastPropertyVector3, values...)
}

// VertexTangentBuffer returns the vertex tangent buffer
func (n *CastNodeMesh) VertexTangentBuffer() []Vec3 {
	return getPropertyValues[Vec3](&n.castNode, CastPropertyNameVertexTangentBuffer)
}

// SetVertexTangentBuffer sets the vertex tangent buffer
func (n *CastNodeMesh) SetVertexTangentBuffer(values []Vec3) {
	createProperty(&n.castNode, CastPropertyNameVertexTangentBuffer, CastPropertyVector3, values...)
}

// VertexColorBuffer returns the vertex color buffer
func (n *CastNodeMesh) VertexColorBuffer() []uint32 {
	return getPropertyValues[uint32](&n.castNode, CastPropertyNameVertexColorBuffer)
}

// SetVertexColorBuffer sets the vertex color buffer
func (n *CastNodeMesh) SetVertexColorBuffer(values []uint32) {
	createProperty(&n.castNode, CastPropertyNameVertexColorBuffer, CastPropertyInteger32, values...)
}

// VertexUVLayerBuffer returns the vertex UV layer buffer for the given index
func (n *CastNodeMesh) VertexUVLayerBuffer(index int) []Vec2 {
	return getPropertyValues[Vec2](&n.castNode, fmt.Sprintf(CastPropertyNameVertexUVBuffer, index))
}

// SetVertexUVLayerBuffer sets the UV layer buffer for the given index
func (n *CastNodeMesh) SetVertexUVLayerBuffer(index int, values []Vec2) {
	createProperty(&n.castNode, fmt.Sprintf(CastPropertyNameVertexUVBuffer, index), CastPropertyVector2, values...)
}

// VertexWeightBoneBuffer returns the vertex weight bone buffer
func (n *CastNodeMesh) VertexWeightBoneBuffer() []uint32 {
	// TODO
	return getPropertyValues[uint32](&n.castNode, "wb")
}

// SetVertexWeightBoneBuffer sets the vertex weight buffer
func (n *CastNodeMesh) SetVertexWeightBoneBuffer(values []uint32) {
	createProperty(&n.castNode, "wb", CastPropertyInteger32, values...)
}

// VertexWeightValueBuffer returns the vertex weight value buffer
func (n *CastNodeMesh) VertexWeightValueBuffer() []float32 {
	return getPropertyValues[float32](&n.castNode, CastPropertyNameVertexWeightBoneBuffer)
}

// SetVertexWeightValueBuffer sets the vertex weight value buffer
func (n *CastNodeMesh) SetVertexWeightValueBuffer(values []float32) {
	createProperty(&n.castNode, CastPropertyNameVertexWeightBoneBuffer, CastPropertyFloat, values...)
}

// Material returns the material node
func (n *CastNodeMesh) Material() *CastNodeMaterial {
	return getChildByHash[CastNodeMaterial](n, CastPropertyNameMaterial, false)
}

// SetMaterial sets the material node by hash
func (n *CastNodeMesh) SetMaterial(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameMaterial, CastPropertyInteger64, hash)
}

type CastNodeBlendShape struct{ castNode }

// NewCastNodeBlendShape creates a new blendshape node
func NewCastNodeBlendShape() *CastNodeBlendShape {
	return NewCastNode[CastNodeBlendShape]()
}

// Name returns the name property
func (n *CastNodeBlendShape) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeBlendShape) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// BaseShape returns the base shape mesh node
func (n *CastNodeBlendShape) BaseShape() *CastNodeMesh {
	return getChildByHash[CastNodeMesh](n, CastPropertyNameBaseShape, false)
}

// SetBaseShape sets the base shape mesh node by hash
func (n *CastNodeBlendShape) SetBaseShape(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameBaseShape, CastPropertyInteger64, hash)
}

// TargetShapes returns the target shape mesh nodes
func (n *CastNodeBlendShape) TargetShapes() []*CastNodeMesh {
	meshHashes := getPropertyValues[uint64](&n.castNode, CastPropertyNameTargetShape)
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

// SetTargetShapes sets the target shapes by hashes
func (n *CastNodeBlendShape) SetTargetShapes(hashes []uint64) {
	createProperty(&n.castNode, CastPropertyNameTargetShape, CastPropertyInteger64, hashes...)
}

// TargetWeightScales returns the target weight scales
func (n *CastNodeBlendShape) TargetWeightScales() []float32 {
	return getPropertyValues[float32](&n.castNode, CastPropertyNameTargetWeightScale)
}

// SetTargetWeightScales sets the target weight scales
func (n *CastNodeBlendShape) SetTargetWeightScales(values []float32) {
	createProperty(&n.castNode, CastPropertyNameTargetWeightScale, CastPropertyFloat, values...)
}

type CastNodeSkeleton struct{ castNode }

// NewCastNodeSkeleton creates a new skeleton node
func NewCastNodeSkeleton() *CastNodeSkeleton {
	return NewCastNode[CastNodeSkeleton]()
}

// Bones returns the bone nodes
func (n *CastNodeSkeleton) Bones() []*CastNodeBone {
	return getChildrenOfType[CastNodeBone](&n.castNode, CastIdBone)
}

// CreateBone creates a new bone node
func (n *CastNodeSkeleton) CreateBone() *CastNodeBone {
	return (n.CreateChild(NewCastNodeBone())).(*CastNodeBone)
}

// IKHandles returns the IK handle nodes
func (n *CastNodeSkeleton) IKHandles() []*CastNodeIKHandle {
	return getChildrenOfType[CastNodeIKHandle](&n.castNode, CastIdIKHandle)
}

// CreateIKHandle creates a new IK handle node
func (n *CastNodeSkeleton) CreateIKHandle() *CastNodeIKHandle {
	return (n.CreateChild(NewCastNodeIKHandle())).(*CastNodeIKHandle)
}

// Constraints returns the constraint nodes
func (n *CastNodeSkeleton) Constraints() []*CastNodeConstraint {
	return getChildrenOfType[CastNodeConstraint](&n.castNode, CastIdConstraint)
}

// CreateConstraint creates a new constraint node
func (n *CastNodeSkeleton) CreateConstraint() *CastNodeConstraint {
	return (n.CreateChild(NewCastNodeConstraint())).(*CastNodeConstraint)
}

type CastNodeBone struct{ castNode }

// NewCastNodeBone returns a new bone node
func NewCastNodeBone() *CastNodeBone {
	return NewCastNode[CastNodeBone]()
}

// Name returns the name property
func (n *CastNodeBone) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeBone) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// ParentIndex returns the parent index
func (n *CastNodeBone) ParentIndex() int32 {
	values := getPropertyValues[uint32](&n.castNode, CastPropertyNameParentIndex)
	if len(values) == 0 {
		return -1
	}
	return int32((values[0] & 0xFFFFFFFF) ^ 0x80000000 - 0x80000000)
}

// SetParentIndex sets the parent index
func (n *CastNodeBone) SetParentIndex(index int32) {
	idx := uint32(index)
	if index < 0 {
		idx = uint32(index) + uint32(math.Pow(2, 32))
	}
	createProperty(&n.castNode, CastPropertyNameParentIndex, CastPropertyInteger32, idx)
}

// SegmentScaleCompensate returns if the node uses segment scale compensation
func (n *CastNodeBone) SegmentScaleCompensate() bool {
	values := getPropertyValues[byte](&n.castNode, CastPropertyNameSegmentScaleCompensate)
	if len(values) == 0 {
		return true
	}

	return values[0] >= 1
}

// SetSegmentScaleCompensate sets segment scale compensation
func (n *CastNodeBone) SetSegmentScaleCompensate(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.castNode, CastPropertyNameSegmentScaleCompensate, CastPropertyByte, value)
}

// LocalPosition returns the local position
func (n *CastNodeBone) LocalPosition() Vec3 {
	return getPropertyValue[Vec3](&n.castNode, CastPropertyNameLocalPosition)
}

// SetLocalPosition sets the local position
func (n *CastNodeBone) SetLocalPosition(position Vec3) {
	createProperty(&n.castNode, CastPropertyNameLocalPosition, CastPropertyVector3, position)
}

// LocalRotation returns the local rotation
func (n *CastNodeBone) LocalRotation() Vec4 {
	return getPropertyValue[Vec4](&n.castNode, CastPropertyNameLocalRotation)
}

// SetLocalRotation sets the local rotation
func (n *CastNodeBone) SetLocalRotation(rotation Vec4) {
	createProperty(&n.castNode, CastPropertyNameLocalRotation, CastPropertyVector4, rotation)
}

// WorldPosition sets the world position
func (n *CastNodeBone) WorldPosition() Vec3 {
	return getPropertyValue[Vec3](&n.castNode, CastPropertyNameWorldPosition)
}

// SetWorldPosition sets the world position
func (n *CastNodeBone) SetWorldPosition(position Vec3) {
	createProperty(&n.castNode, CastPropertyNameWorldPosition, CastPropertyVector3, position)
}

// WorldRotation returns the world rotation
func (n *CastNodeBone) WorldRotation() Vec4 {
	return getPropertyValue[Vec4](&n.castNode, CastPropertyNameWorldRotation)
}

// SetWorldRotation sets the world rotation
func (n *CastNodeBone) SetWorldRotation(rotation Vec4) {
	createProperty(&n.castNode, CastPropertyNameWorldRotation, CastPropertyVector4, rotation)
}

// Scale returns the scale
func (n *CastNodeBone) Scale() Vec3 {
	return getPropertyValue[Vec3](&n.castNode, CastPropertyNameScale)
}

// SetScale sets the scale
func (n *CastNodeBone) SetScale(scale Vec3) {
	createProperty(&n.castNode, CastPropertyNameScale, CastPropertyVector3, scale)
}

type CastNodeIKHandle struct{ castNode }

// NewCastNodeIKHandle creates a new IK handle node
func NewCastNodeIKHandle() *CastNodeIKHandle {
	return NewCastNode[CastNodeIKHandle]()
}

// Name returns the name property
func (n *CastNodeIKHandle) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeIKHandle) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// StartBone returns the start bone node
func (n *CastNodeIKHandle) StartBone() *CastNodeBone {
	return getChildByHash[CastNodeBone](n, CastPropertyNameStartBone, true)
}

// SetStartBone sets the start bone by hash
func (n *CastNodeIKHandle) SetStartBone(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameStartBone, CastPropertyInteger64, hash)
}

// EndBone returns the end bone node
func (n *CastNodeIKHandle) EndBone() *CastNodeBone {
	return getChildByHash[CastNodeBone](n, CastPropertyNameEndBone, true)
}

// SetEndBone sets the end bone by hash
func (n *CastNodeIKHandle) SetEndBone(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameEndBone, CastPropertyInteger64, hash)
}

// TargetBone returns the target bone node
func (n *CastNodeIKHandle) TargetBone() *CastNodeBone {
	return getChildByHash[CastNodeBone](n, CastPropertyNameTargetBone, true)
}

// SetTargetBone sets the target bone by hash
func (n *CastNodeIKHandle) SetTargetBone(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameTargetBone, CastPropertyInteger64, hash)
}

// PoleVectorBone returns the pole vector bone node
func (n *CastNodeIKHandle) PoleVectorBone() *CastNodeBone {
	return getChildByHash[CastNodeBone](n, CastPropertyNamePoleVectorBone, true)
}

// SetPoleVectorBone sets the pole vector bone by hash
func (n *CastNodeIKHandle) SetPoleVectorBone(hash uint64) {
	createProperty(&n.castNode, CastPropertyNamePoleVectorBone, CastPropertyInteger64, hash)
}

// PoleBone returns the pole bone node
func (n *CastNodeIKHandle) PoleBone() *CastNodeBone {
	return getChildByHash[CastNodeBone](n, CastPropertyNamePoleBone, true)
}

// SetPoleBone sets the pole bone node by hash
func (n *CastNodeIKHandle) SetPoleBone(hash uint64) {
	createProperty(&n.castNode, CastPropertyNamePoleBone, CastPropertyInteger64, hash)
}

// UseTargetRotation returns if the node uses target rotation
func (n *CastNodeIKHandle) UseTargetRotation() bool {
	values := getPropertyValues[byte](&n.castNode, CastPropertyNameTargetRotation)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

// SetUseTargetRotation sets the the target rotation
func (n *CastNodeIKHandle) SetUseTargetRotation(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.castNode, CastPropertyNameTargetRotation, CastPropertyByte, value)
}

type CastNodeConstraint struct{ castNode }

// NewCastNodeConstraint creates a new constraint node
func NewCastNodeConstraint() *CastNodeConstraint {
	return NewCastNode[CastNodeConstraint]()
}

// Name returns the name property
func (n *CastNodeConstraint) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeConstraint) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// ConstraintType returns the constraint type
func (n *CastNodeConstraint) ConstraintType() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameConstraintType)
}

// SetConstraintType sets the contstrain type
func (n *CastNodeConstraint) SetConstraintType(constraintType string) {
	createProperty(&n.castNode, CastPropertyNameConstraintType, CastPropertyString, constraintType)
}

// ConstraintBone returns the constraint bone node
func (n *CastNodeConstraint) ConstraintBone() *CastNodeBone {
	return getChildByHash[CastNodeBone](n, CastPropertyNameConstraintBone, true)
}

// SetConstraintBone sets the constraint bone by hash
func (n *CastNodeConstraint) SetConstraintBone(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameConstraintBone, CastPropertyInteger64, hash)
}

// TargetBone returns the target bone node
func (n *CastNodeConstraint) TargetBone() *CastNodeBone {
	return getChildByHash[CastNodeBone](n, CastPropertyNameTargetBone, true)
}

// SetTargetBone sets the target bone by hash
func (n *CastNodeConstraint) SetTargetBone(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameTargetBone, CastPropertyInteger64, hash)
}

// MaintainOffset returns if the node maintains offset
func (n *CastNodeConstraint) MaintainOffset() bool {
	values := getPropertyValues[byte](&n.castNode, CastPropertyNameMaintainOffset)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

// SetMaintainOffset sets the node to maintain offset
func (n *CastNodeConstraint) SetMaintainOffset(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.castNode, CastPropertyNameMaintainOffset, CastPropertyByte, value)
}

// SkipX returns if the node skips X
func (n *CastNodeConstraint) SkipX() bool {
	values := getPropertyValues[byte](&n.castNode, CastPropertyNameSkipX)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

// SetSkipX sets whether to skip X
func (n *CastNodeConstraint) SetSkipX(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.castNode, CastPropertyNameSkipX, CastPropertyByte, value)
}

// SkipY returns if the node skips Y
func (n *CastNodeConstraint) SkipY() bool {
	values := getPropertyValues[byte](&n.castNode, CastPropertyNameSkipY)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

// SetSkipY sets whether to skip Y
func (n *CastNodeConstraint) SetSkipY(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.castNode, CastPropertyNameSkipY, CastPropertyByte, value)
}

// SkipZ returns if the node skips Z
func (n *CastNodeConstraint) SkipZ() bool {
	values := getPropertyValues[byte](&n.castNode, CastPropertyNameSkipZ)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

// SetSkipZ sets whether to skip Z
func (n *CastNodeConstraint) SetSkipZ(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.castNode, CastPropertyNameSkipZ, CastPropertyByte, value)
}

type CastNodeMaterial struct{ castNode }

// NewCastNodeMaterial creates a new material node
func NewCastNodeMaterial() *CastNodeMaterial {
	return NewCastNode[CastNodeMaterial]()
}

// Name returns the name property
func (n *CastNodeMaterial) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeMaterial) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// Type returns the material type
func (n *CastNodeMaterial) Type() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameType)
}

// SetType sets the material type
func (n *CastNodeMaterial) SetType(materialType string) {
	createProperty(&n.castNode, CastPropertyNameType, CastPropertyString, materialType)
}

// Slots returns the material slot files
func (n *CastNodeMaterial) Slots() map[string]*CastNodeFile {
	slots := make(map[string]*CastNodeFile, 0)
	for slot, property := range n.properties {
		if slot != CastPropertyNameName && slot != CastPropertyNameType {
			p, ok := property.(*castProperty[uint64])
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

// CreateFile creates a new file node
func (n *CastNodeMaterial) CreateFile() *CastNodeFile {
	return (n.CreateChild(NewCastNodeFile())).(*CastNodeFile)
}

type CastNodeFile struct{ castNode }

// NewCastNodeFile creates a new file node
func NewCastNodeFile() *CastNodeFile {
	return NewCastNode[CastNodeFile]()
}

// Path returns the path of the file
func (n *CastNodeFile) Path() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNamePath)
}

// SetPath sets the path of the file
func (n *CastNodeFile) SetPath(path string) {
	createProperty(&n.castNode, CastPropertyNamePath, CastPropertyString, path)
}

type CastNodeAnimation struct{ castNode }

// NewCastNodeAnimation creates a new animation node
func NewCastNodeAnimation() *CastNodeAnimation {
	return NewCastNode[CastNodeAnimation]()
}

// Name returns the name property
func (n *CastNodeAnimation) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeAnimation) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// Skeleton returns the skeleton node
func (n *CastNodeAnimation) Skeleton() *CastNodeSkeleton {
	return getChildOfType[CastNodeSkeleton](&n.castNode, CastIdSkeleton)
}

// CreateSkeleton creates a new skeleton node
func (n *CastNodeAnimation) CreateSkeleton() *CastNodeSkeleton {
	return (n.CreateChild(NewCastNodeSkeleton())).(*CastNodeSkeleton)
}

// Curves returns the curves
func (n *CastNodeAnimation) Curves() []*CastNodeCurve {
	return getChildrenOfType[CastNodeCurve](&n.castNode, CastIdCurve)
}

// CreateCurve creates a new curve node
func (n *CastNodeAnimation) CreateCurve() *CastNodeCurve {
	return (n.CreateChild(NewCastNodeCurve())).(*CastNodeCurve)
}

// Notifications returns the notification tracks
func (n *CastNodeAnimation) Notifications() []*CastNodeNotificationTrack {
	return getChildrenOfType[CastNodeNotificationTrack](&n.castNode, CastIdNotificationTrack)
}

// CreateNotification creates a new notification track k
func (n *CastNodeAnimation) CreateNotification() *CastNodeNotificationTrack {
	return (n.CreateChild(NewCastNodeNotificationTrack())).(*CastNodeNotificationTrack)
}

// Framerate returns the framerate
func (n *CastNodeAnimation) Framerate() float32 {
	return getPropertyValue[float32](&n.castNode, CastPropertyNameFramerate)
}

// SetFramerate sets the framerate
func (n *CastNodeAnimation) SetFramerate(framerate float32) {
	createProperty(&n.castNode, CastPropertyNameFramerate, CastPropertyFloat, framerate)
}

// Looping returns whether the animation uses looping
func (n *CastNodeAnimation) Looping() bool {
	values := getPropertyValues[byte](&n.castNode, CastPropertyNameLoop)
	if len(values) == 0 {
		return false
	}

	return values[0] >= 1
}

// SetLooping sets the looping
func (n *CastNodeAnimation) SetLooping(enabled bool) {
	value := byte(0)
	if enabled {
		value = byte(1)
	}
	createProperty(&n.castNode, CastPropertyNameLoop, CastPropertyByte, value)
}

type CastNodeCurve struct{ castNode }

// NewCastNodeCurve creates a new curve node
func NewCastNodeCurve() *CastNodeCurve {
	return NewCastNode[CastNodeCurve]()
}

// NodeName returns the name of the node
func (n *CastNodeCurve) NodeName() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameNodeName)
}

// SetNodeName sets the name of the node
func (n *CastNodeCurve) SetNodeName(name string) {
	createProperty(&n.castNode, CastPropertyNameNodeName, CastPropertyString, name)
}

// KeyPropertyName returns the key property name
func (n *CastNodeCurve) KeyPropertyName() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameKeyProperty)
}

// SetKeyPropertyName sets the key property name
func (n *CastNodeCurve) SetKeyPropertyName(name string) {
	createProperty(&n.castNode, CastPropertyNameKeyProperty, CastPropertyString, name)
}

// KeyFrameBuffer returns the keyframe buffer
func (n *CastNodeCurve) KeyFrameBuffer() []uint32 {
	// TODO
	return getPropertyValues[uint32](&n.castNode, CastPropertyNameKeyFrameBuffer)
}

// SetKeyFrameBuffer sets the keyframe buffer
func (n *CastNodeCurve) SetKeyFrameBuffer(values []uint32) {
	createProperty(&n.castNode, CastPropertyNameKeyFrameBuffer, CastPropertyInteger32, values...)
}

// KeyValueBuffer returns the key-value buffer
func (n *CastNodeCurve) KeyValueBuffer() []uint32 {
	// TODO
	return getPropertyValues[uint32](&n.castNode, CastPropertyNameKeyValueBuffer)
}

// SetFloatKeyValueBuffer sets the float key-value buffer
func (n *CastNodeCurve) SetFloatKeyValueBuffer(values []float32) {
	createProperty(&n.castNode, CastPropertyNameKeyValueBuffer, CastPropertyFloat, values...)
}

// SetVec4KeyValueBuffer sets the vec4 key-value buffer
func (n *CastNodeCurve) SetVec4KeyValueBuffer(values []Vec4) {
	createProperty(&n.castNode, CastPropertyNameKeyValueBuffer, CastPropertyVector4, values...)
}

// SetByteKeyValueBuffer sets the byte key-value buffer
func (n *CastNodeCurve) SetByteKeyValueBuffer(values []byte) {
	createProperty(&n.castNode, CastPropertyNameKeyValueBuffer, CastPropertyByte, values...)
}

// Mode returns the mode
func (n *CastNodeCurve) Mode() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameMode)
}

// SetMode sets the mode
func (n *CastNodeCurve) SetMode(mode string) {
	createProperty(&n.castNode, CastPropertyNameMode, CastPropertyString, mode)
}

// AdditiveBlendWeight returns the additive blend weight
func (n *CastNodeCurve) AdditiveBlendWeight() float32 {
	values := getPropertyValues[float32](&n.castNode, CastPropertyNameAdditiveBlendWeight)
	if len(values) == 0 {
		return 1.0
	}
	return values[0]
}

// SetAdditiveBlendWeight sets the additive blend weight
func (n *CastNodeCurve) SetAdditiveBlendWeight(value float32) {
	createProperty(&n.castNode, CastPropertyNameAdditiveBlendWeight, CastPropertyFloat, value)
}

type CastNodeNotificationTrack struct{ castNode }

// NewCastNodeNotificationTrack creates a new notification track node
func NewCastNodeNotificationTrack() *CastNodeNotificationTrack {
	return NewCastNode[CastNodeNotificationTrack]()
}

// Name returns the name property
func (n *CastNodeNotificationTrack) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeNotificationTrack) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// KeyFrameBuffer returns the keyframe buffer
func (n *CastNodeNotificationTrack) KeyFrameBuffer() []uint32 {
	// TODO
	return getPropertyValues[uint32](&n.castNode, CastPropertyNameKeyFrameBuffer)
}

// SetKeyFrameBuffer sets the keyframe buffer
func (n *CastNodeNotificationTrack) SetKeyFrameBuffer(values []uint32) {
	createProperty(&n.castNode, CastPropertyNameKeyFrameBuffer, CastPropertyInteger32, values...)
}

type CastNodeInstance struct{ castNode }

// NewCastNodeInstance creates a new node instance
func NewCastNodeInstance() *CastNodeInstance {
	return NewCastNode[CastNodeInstance]()
}

// Name returns the name property
func (n *CastNodeInstance) Name() string {
	return getPropertyValue[string](&n.castNode, CastPropertyNameName)
}

// SetName sets the name property
func (n *CastNodeInstance) SetName(name string) {
	createProperty(&n.castNode, CastPropertyNameName, CastPropertyString, name)
}

// ReferenceFile returns the reference file
func (n *CastNodeInstance) ReferenceFile() *CastNodeFile {
	return getChildByHash[CastNodeFile](n, CastPropertyNameReferenceFile, false)
}

// SetReferenceFile sets the reference file
func (n *CastNodeInstance) SetReferenceFile(hash uint64) {
	createProperty(&n.castNode, CastPropertyNameReferenceFile, CastPropertyInteger64, hash)
}

// Position returns the position
func (n *CastNodeInstance) Position() []Vec3 {
	return getPropertyValues[Vec3](&n.castNode, CastPropertyNamePosition)
}

// SetPosition sets the position
func (n *CastNodeInstance) SetPosition(position Vec3) {
	createProperty(&n.castNode, CastPropertyNamePosition, CastPropertyVector3, position)
}

// Rotation returns the rotation
func (n *CastNodeInstance) Rotation() []Vec4 {
	return getPropertyValues[Vec4](&n.castNode, CastPropertyNamePosition)
}

// SetRotation sets the rotation
func (n *CastNodeInstance) SetRotation(rotation Vec4) {
	createProperty(&n.castNode, CastPropertyNamePosition, CastPropertyVector4, rotation)
}

// Scale returns the scale
func (n *CastNodeInstance) Scale() []Vec3 {
	return getPropertyValues[Vec3](&n.castNode, CastPropertyNameScale)
}

// SetScale sets the scale
func (n *CastNodeInstance) SetScale(scale Vec3) {
	createProperty(&n.castNode, CastPropertyNameScale, CastPropertyVector3, scale)
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
	CastPropertyNameFramerate              = "fr"
	CastPropertyNameLoop                   = "lo"
	CastPropertyNameNodeName               = "nn"
	CastPropertyNameKeyProperty            = "kp"
	CastPropertyNameKeyFrameBuffer         = "kb"
	CastPropertyNameKeyValueBuffer         = "kv"
	CastPropertyNameMode                   = "m"
	CastPropertyNameAdditiveBlendWeight    = "ab"
	CastPropertyNameReferenceFile          = "rf"
	CastPropertyNamePosition               = "p"
	CastPropertyNameRotation               = "r"
)

// castPropertyHeader holds header data of the property
type castPropertyHeader struct {
	Identifier  CastPropertyId
	NameSize    uint16
	ArrayLength uint32
}

// iCastProperty is the property interface
type iCastProperty interface {
	Name() string
	Identifier() CastPropertyId
	Load(r io.Reader) error
	Length() int
	Write(w io.Writer) error
	ValueCount() int
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

	property, err := newCastProperty(header.Identifier, string(name), header.ArrayLength)
	if err != nil {
		return nil, err
	}

	if err := property.Load(r); err != nil {
		return nil, err
	}

	return property, nil
}

// newCastProperty creates a new property with the given type, name and size
func newCastProperty(id CastPropertyId, name string, size uint32) (iCastProperty, error) {
	switch id {
	case CastPropertyByte:
		return &castProperty[byte]{
			id:     id,
			name:   name,
			values: make([]byte, size),
		}, nil
	case CastPropertyShort:
		return &castProperty[uint16]{
			id:     id,
			name:   name,
			values: make([]uint16, size),
		}, nil
	case CastPropertyInteger32:
		return &castProperty[uint32]{
			id:     id,
			name:   name,
			values: make([]uint32, size),
		}, nil
	case CastPropertyInteger64:
		return &castProperty[uint64]{
			id:     id,
			name:   name,
			values: make([]uint64, size),
		}, nil
	case CastPropertyFloat:
		return &castProperty[float32]{
			id:     id,
			name:   name,
			values: make([]float32, size),
		}, nil
	case CastPropertyDouble:
		return &castProperty[float64]{
			id:     id,
			name:   name,
			values: make([]float64, size),
		}, nil
	case CastPropertyString:
		return &castProperty[string]{
			id:     id,
			name:   name,
			values: make([]string, size),
		}, nil
	case CastPropertyVector2:
		return &castProperty[Vec2]{
			id:     id,
			name:   name,
			values: make([]Vec2, size),
		}, nil
	case CastPropertyVector3:
		return &castProperty[Vec3]{
			id:     id,
			name:   name,
			values: make([]Vec3, size),
		}, nil

	case CastPropertyVector4:
		return &castProperty[Vec4]{
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

// castProperty holds data of a property
type castProperty[T CastPropertyValueType] struct {
	id     CastPropertyId
	name   string
	values []T
}

// Name returns the name
func (p *castProperty[T]) Name() string {
	return p.name
}

// Identifier returns the property id
func (p *castProperty[T]) Identifier() CastPropertyId {
	return p.id
}

// Load loads a property from the given [io.Reader]
func (p *castProperty[T]) Load(r io.Reader) error {
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

// Write writes a property to the given [io.Writer]
func (p *castProperty[T]) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, castPropertyHeader{
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

// Length returns the length of the property
func (p *castProperty[T]) Length() int {
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

// ValueCount returns the amount of values held by the property
func (p *castProperty[T]) ValueCount() int {
	return len(p.values)
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

// createProperty creates a new property on the given node
func createProperty[T CastPropertyValueType](node *castNode, name string, id CastPropertyId, values ...T) {
	property, _ := node.CreateProperty(name, id)
	p := property.(*castProperty[T])
	p.values = append(p.values, values...)
}

// getPropertyValues returns the property values of the given node
func getPropertyValues[T CastPropertyValueType](node *castNode, name string) []T {
	// TODO

	property, ok := node.GetProperty(name)
	if !ok {
		return []T{}
	}

	p, ok := property.(*castProperty[T])
	if !ok {
		return []T{}
	}

	return p.values
}

// getPropertyValue returns a property value of the given node
func getPropertyValue[T CastPropertyValueType](node *castNode, name string) T {
	values := getPropertyValues[T](node, name)
	if len(values) == 0 {
		return *new(T)
	}
	return values[0]
}

// getChildOfType returns a childnode of the given node and type
func getChildOfType[T CastNodes](node *castNode, id CastId) *T {
	children := node.ChildrenOfType(id)
	if len(children) == 0 {
		return nil
	}

	child, ok := (children[0]).(T)
	if !ok {
		return nil
	}

	return &child
}

// getChildrenOfType returns the childnodes of the given node and type
func getChildrenOfType[T CastNodes](node *castNode, id CastId) []*T {
	c := node.ChildrenOfType(id)
	children := make([]*T, len(c))

	for i := range children {
		cc, ok := (c[i]).(T)
		if !ok {
			continue
		}

		children[i] = &cc
	}

	return children
}

// getChildByHash returns a childnode of the given node by hash
func getChildByHash[T CastNodes](node iCastNode, name string, useParent bool) *T {
	values := getPropertyValues[uint64](node.GetCastNode(), name)
	if len(values) == 0 {
		return nil
	}

	nn := node
	if useParent {
		nn = node.GetParentNode()
		if nn == nil {
			return nil
		}
	}

	c := nn.ChildByHash(values[0])
	if c == nil {
		return nil
	}

	child, ok := c.(T)
	if !ok {
		return nil
	}

	return &child
}
