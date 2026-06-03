// Package nrc — Cobra Annotations 常量定义。
//
// main.go 在构建 cobra.Command 时把这些 Annotation 字符串塞入 cmd.Annotations,
// 供 installRiskHelpFunc 与未来 AI 调度器读取。
package nrc

const (
	AnnotationRisk      = "inl.dev/risk"
	AnnotationPureGroup = "inl.dev/pure-group"
	AnnotationDataType  = "inl.dev/datatype"
	AnnotationFunction  = "inl.dev/function"
)
