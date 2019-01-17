// Copyright 2015 Peter Goetz
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Based on the work done in
// https://github.com/golang/mock/blob/d581abfc04272f381d7a05e4b80163ea4e2b9447/mockgen/mockgen.go

// MockGen generates mock implementations of Go interfaces.
package mockgen

// TODO: This does not support recursive embedded interfaces.
// TODO: This does not support embedding package-local interfaces in a separate file.

import (
	"bytes"
	"fmt"
	"go/format"
	"go/token"
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/petergtz/pegomock/model"
)

const mockFrameworkImportPath = "github.com/petergtz/pegomock"

func GenerateOutput(ast *model.Package, source, packageOut, selfPackage string) ([]byte, map[string]string) {
	g := generator{typesSet: make(map[string]string)}
	g.generateCode(source, ast, packageOut, selfPackage)
	return g.formattedOutput(), g.typesSet
}

type generator struct {
	buf        bytes.Buffer
	packageMap map[string]string // map from import path to package name
	typesSet   map[string]string
}

func (g *generator) generateCode(source string, pkg *model.Package, pkgName, selfPackage string) {
	g.p("// Code generated by pegomock. DO NOT EDIT.")
	g.p("// Source: %v", source)
	g.emptyLine()

	importPaths := pkg.Imports()
	importPaths[mockFrameworkImportPath] = true
	packageMap, nonVendorPackageMap := generateUniquePackageNamesFor(importPaths)
	g.packageMap = packageMap

	g.p("package %v", pkgName)
	g.emptyLine()
	g.p("import (")
	g.p("\"reflect\"")
	g.p("\"time\"")
	for packagePath, packageName := range nonVendorPackageMap {
		if packagePath != selfPackage && packagePath != "time" && packagePath != "reflect" {
			g.p("%v %q", packageName, packagePath)
		}
	}
	for _, packagePath := range pkg.DotImports {
		g.p(". %q", packagePath)
	}
	g.p(")")

	for _, iface := range pkg.Interfaces {
		g.generateMockFor(iface, selfPackage)
	}
}

func generateUniquePackageNamesFor(importPaths map[string]bool) (packageMap, nonVendorPackageMap map[string]string) {
	packageMap = make(map[string]string, len(importPaths))
	nonVendorPackageMap = make(map[string]string, len(importPaths))
	packageNamesAlreadyUsed := make(map[string]bool, len(importPaths))
	for importPath := range importPaths {
		sanitizedPackagePathBaseName := sanitize(path.Base(importPath))

		// Local names for an imported package can usually be the basename of the import path.
		// A couple of situations don't permit that, such as duplicate local names
		// (e.g. importing "html/template" and "text/template"), or where the basename is
		// a keyword (e.g. "foo/case").
		// try base0, base1, ...
		packageName := sanitizedPackagePathBaseName
		for i := 0; packageNamesAlreadyUsed[packageName] || token.Lookup(packageName).IsKeyword(); i++ {
			packageName = sanitizedPackagePathBaseName + strconv.Itoa(i)
		}

		packageMap[importPath] = packageName
		packageNamesAlreadyUsed[packageName] = true

		nonVendorPackageMap[vendorCleaned(importPath)] = packageName
	}
	return
}

func vendorCleaned(importPath string) string {
	if split := strings.Split(importPath, "/vendor/"); len(split) > 1 {
		return split[1]
	}
	return importPath
}

// sanitize cleans up a string to make a suitable package name.
// pkgName in reflect mode is the base name of the import path,
// which might have characters that are illegal to have in package names.
func sanitize(s string) string {
	t := ""
	for _, r := range s {
		if t == "" {
			if unicode.IsLetter(r) || r == '_' {
				t += string(r)
				continue
			}
		} else {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				t += string(r)
				continue
			}
		}
		t += "_"
	}
	if t == "_" {
		t = "x"
	}
	return t
}

func (g *generator) generateMockFor(iface *model.Interface, selfPackage string) {
	mockTypeName := "Mock" + iface.Name
	g.generateMockType(mockTypeName)
	for _, method := range iface.Methods {
		g.generateMockMethod(mockTypeName, method, selfPackage)
		g.emptyLine()

		addTypesFromMethodParamsTo(g.typesSet, method.In, g.packageMap)
		addTypesFromMethodParamsTo(g.typesSet, method.Out, g.packageMap)
	}
	g.generateMockVerifyMethods(iface.Name)
	g.generateVerifierType(iface.Name)
	for _, method := range iface.Methods {
		ongoingVerificationTypeName := fmt.Sprintf("%v_%v_OngoingVerification", iface.Name, method.Name)
		args, argNames, argTypes, _ := argDataFor(method, g.packageMap, selfPackage)
		g.generateVerifierMethod(iface.Name, method, selfPackage, ongoingVerificationTypeName, args, argNames)
		g.generateOngoingVerificationType(iface.Name, ongoingVerificationTypeName)
		g.generateOngoingVerificationGetCapturedArguments(ongoingVerificationTypeName, argNames, argTypes)
		g.generateOngoingVerificationGetAllCapturedArguments(ongoingVerificationTypeName, argTypes, method.Variadic != nil)
	}
}

func (g *generator) generateMockType(mockTypeName string) {
	g.
		emptyLine().
		p("type %v struct {", mockTypeName).
		p("	fail func(message string, callerSkip ...int)").
		p("}").
		emptyLine().
		p("func New%v(options ...pegomock.Option) *%v {", mockTypeName, mockTypeName).
		p("	mock := &%v{}", mockTypeName).
		p("	for _, option := range options {").
		p("		option.Apply(mock)").
		p("	}").
		p("	return mock").
		p("}").
		emptyLine().
		p("func (mock *%v) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }", mockTypeName).
		p("func (mock *%v) FailHandler() pegomock.FailHandler      { return mock.fail }", mockTypeName).
		emptyLine()
}

// If non-empty, pkgOverride is the package in which unqualified types reside.
func (g *generator) generateMockMethod(mockType string, method *model.Method, pkgOverride string) *generator {
	args, argNames, _, returnTypes := argDataFor(method, g.packageMap, pkgOverride)
	g.p("func (mock *%v) %v(%v) (%v) {", mockType, method.Name, join(args), join(returnTypes))
	g.p("if mock == nil {").
		p("	panic(\"mock must not be nil. Use myMock := New%v().\")", mockType).
		p("}")
	g.GenerateParamsDeclaration(argNames, method.Variadic != nil)
	reflectReturnTypes := make([]string, len(returnTypes))
	for i, returnType := range returnTypes {
		reflectReturnTypes[i] = fmt.Sprintf("reflect.TypeOf((*%v)(nil)).Elem()", returnType)
	}
	resultAssignment := ""
	if len(method.Out) > 0 {
		resultAssignment = "result :="
	}
	g.p("%v pegomock.GetGenericMockFrom(mock).Invoke(\"%v\", params, []reflect.Type{%v})",
		resultAssignment, method.Name, strings.Join(reflectReturnTypes, ", "))
	if len(method.Out) > 0 {
		// TODO: translate LastInvocation into a Matcher so it can be used as key for Stubbings
		for i, returnType := range returnTypes {
			g.p("var ret%v %v", i, returnType)
		}
		g.p("if len(result) != 0 {")
		returnValues := make([]string, len(returnTypes))
		for i, returnType := range returnTypes {
			g.p("if result[%v] != nil {", i)
			g.p("ret%v  = result[%v].(%v)", i, i, returnType)
			g.p("}")
			returnValues[i] = fmt.Sprintf("ret%v", i)
		}
		g.p("}")
		g.p("return %v", strings.Join(returnValues, ", "))
	}
	g.p("}")
	return g
}

func (g *generator) generateVerifierType(interfaceName string) *generator {
	return g.
		p("type Verifier%v struct {", interfaceName).
		p("	mock *Mock%v", interfaceName).
		p("	invocationCountMatcher pegomock.Matcher").
		p("	inOrderContext *pegomock.InOrderContext").
		p("	timeout time.Duration").
		p("}").
		emptyLine()
}

func (g *generator) generateMockVerifyMethods(interfaceName string) {
	g.
		p("func (mock *Mock%v) VerifyWasCalledOnce() *Verifier%v {", interfaceName, interfaceName).
		p("	return &Verifier%v{", interfaceName).
		p("		mock: mock,").
		p("		invocationCountMatcher: pegomock.Times(1),").
		p("	}").
		p("}").
		emptyLine().
		p("func (mock *Mock%v) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *Verifier%v {", interfaceName, interfaceName).
		p("	return &Verifier%v{", interfaceName).
		p("		mock: mock,").
		p("		invocationCountMatcher: invocationCountMatcher,").
		p("	}").
		p("}").
		emptyLine().
		p("func (mock *Mock%v) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *Verifier%v {", interfaceName, interfaceName).
		p("	return &Verifier%v{", interfaceName).
		p("		mock: mock,").
		p("		invocationCountMatcher: invocationCountMatcher,").
		p("		inOrderContext: inOrderContext,").
		p("	}").
		p("}").
		emptyLine().
		p("func (mock *Mock%v) VerifyWasCalledEventually(invocationCountMatcher pegomock.Matcher, timeout time.Duration) *Verifier%v {", interfaceName, interfaceName).
		p("	return &Verifier%v{", interfaceName).
		p("		mock: mock,").
		p("		invocationCountMatcher: invocationCountMatcher,").
		p("		timeout: timeout,").
		p("	}").
		p("}").
		emptyLine()
}

func (g *generator) generateVerifierMethod(interfaceName string, method *model.Method, pkgOverride string, returnTypeString string, args []string, argNames []string) *generator {
	return g.
		p("func (verifier *Verifier%v) %v(%v) *%v {", interfaceName, method.Name, join(args), returnTypeString).
		GenerateParamsDeclaration(argNames, method.Variadic != nil).
		p("methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, \"%v\", params, verifier.timeout)", method.Name).
		p("return &%v{mock: verifier.mock, methodInvocations: methodInvocations}", returnTypeString).
		p("}")
}

func (g *generator) GenerateParamsDeclaration(argNames []string, isVariadic bool) *generator {
	if isVariadic {
		return g.
			p("params := []pegomock.Param{%v}", strings.Join(argNames[0:len(argNames)-1], ", ")).
			p("for _, param := range %v {", argNames[len(argNames)-1]).
			p("params = append(params, param)").
			p("}")
	} else {
		return g.p("params := []pegomock.Param{%v}", join(argNames))
	}
}

func (g *generator) generateOngoingVerificationType(interfaceName string, ongoingVerificationStructName string) *generator {
	return g.
		p("type %v struct {", ongoingVerificationStructName).
		p("mock *Mock%v", interfaceName).
		p("	methodInvocations []pegomock.MethodInvocation").
		p("}").
		emptyLine()
}

func (g *generator) generateOngoingVerificationGetCapturedArguments(ongoingVerificationStructName string, argNames []string, argTypes []string) *generator {
	g.p("func (c *%v) GetCapturedArguments() (%v) {", ongoingVerificationStructName, join(argTypes))
	if len(argNames) > 0 {
		indexedArgNames := make([]string, len(argNames))
		for i, argName := range argNames {
			indexedArgNames[i] = argName + "[len(" + argName + ")-1]"
		}
		g.p("%v := c.GetAllCapturedArguments()", join(argNames))
		g.p("return %v", strings.Join(indexedArgNames, ", "))
	}
	g.p("}")
	g.emptyLine()
	return g
}

func (g *generator) generateOngoingVerificationGetAllCapturedArguments(ongoingVerificationStructName string, argTypes []string, isVariadic bool) *generator {
	argsAsArray := make([]string, len(argTypes))
	for i, argType := range argTypes {
		argsAsArray[i] = fmt.Sprintf("_param%v []%v", i, argType)
	}
	g.p("func (c *%v) GetAllCapturedArguments() (%v) {", ongoingVerificationStructName, strings.Join(argsAsArray, ", "))
	if len(argTypes) > 0 {
		g.p("params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)")
		g.p("if len(params) > 0 {")
		for i, argType := range argTypes {
			if isVariadic && i == len(argTypes)-1 {
				variadicBasicType := strings.Replace(argType, "[]", "", 1)
				g.
					p("_param%v = make([]%v, len(params[%v]))", i, argType, i).
					p("for u := range params[0] {"). // the number of invocations and hence len(params[x]) is equal for all x
					p("_param%v[u] = make([]%v, len(params)-%v)", i, variadicBasicType, i).
					p("for x := %v; x < len(params); x++ {", i).
					p("if params[x][u] != nil {").
					p("_param%v[u][x-%v] = params[x][u].(%v)", i, i, variadicBasicType).
					p("}").
					p("}").
					p("}")
				break
			} else {
				g.p("_param%v = make([]%v, len(params[%v]))", i, argType, i)
				g.p("for u, param := range params[%v] {", i)
				g.p("_param%v[u]=param.(%v)", i, argType)
				g.p("}")
			}
		}
		g.p("}")
		g.p("return")
	}
	g.p("}")
	g.emptyLine()
	return g
}

func argDataFor(method *model.Method, packageMap map[string]string, pkgOverride string) (
	args []string,
	argNames []string,
	argTypes []string,
	returnTypes []string,
) {
	args = make([]string, len(method.In))
	argNames = make([]string, len(method.In))
	argTypes = make([]string, len(args))
	for i, arg := range method.In {
		argName := arg.Name
		if argName == "" {
			argName = fmt.Sprintf("_param%d", i)
		}
		argType := arg.Type.String(packageMap, pkgOverride)
		args[i] = argName + " " + argType
		argNames[i] = argName
		argTypes[i] = argType
	}
	if method.Variadic != nil {
		argName := method.Variadic.Name
		if argName == "" {
			argName = fmt.Sprintf("_param%d", len(method.In))
		}
		argType := method.Variadic.Type.String(packageMap, pkgOverride)
		args = append(args, argName+" ..."+argType)
		argNames = append(argNames, argName)
		argTypes = append(argTypes, "[]"+argType)
	}
	returnTypes = make([]string, len(method.Out))
	for i, ret := range method.Out {
		returnTypes[i] = ret.Type.String(packageMap, pkgOverride)
	}
	return
}

func addTypesFromMethodParamsTo(typesSet map[string]string, params []*model.Parameter, packageMap map[string]string) {
	for _, param := range params {
		switch typedType := param.Type.(type) {
		case *model.NamedType, *model.PointerType, *model.ArrayType, *model.MapType, *model.ChanType:
			if _, exists := typesSet[underscoreNameFor(typedType, packageMap)]; !exists {
				typesSet[underscoreNameFor(typedType, packageMap)] = generateMatcherSourceCode(typedType, packageMap)
			}
		case *model.FuncType:
			// matcher generation for funcs not supported yet
			// TODO implement
		case model.PredeclaredType:
			// skip. These come as part of pegomock.
		default:
			panic("Should not get here")
		}
	}
}

func generateMatcherSourceCode(t model.Type, packageMap map[string]string) string {
	return fmt.Sprintf(`// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"
	"github.com/petergtz/pegomock"
	%v
)

func Any%v() %v {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(%v))(nil)).Elem()))
	var nullValue %v
	return nullValue
}

func Eq%v(value %v) %v {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue %v
	return nullValue
}
`,
		optionalPackageOf(t, packageMap),
		camelcaseNameFor(t, packageMap),
		t.String(packageMap, ""),
		t.String(packageMap, ""),
		t.String(packageMap, ""),

		camelcaseNameFor(t, packageMap),
		t.String(packageMap, ""),
		t.String(packageMap, ""),
		t.String(packageMap, ""),
	)
}

func optionalPackageOf(t model.Type, packageMap map[string]string) string {
	switch typedType := t.(type) {
	case model.PredeclaredType:
		return ""
	case *model.NamedType:
		return fmt.Sprintf("%v \"%v\"", packageMap[typedType.Package], vendorCleaned(typedType.Package))
	case *model.PointerType:
		return optionalPackageOf(typedType.Type, packageMap)
	case *model.ArrayType:
		return optionalPackageOf(typedType.Type, packageMap)
	case *model.MapType:
		return optionalPackageOf(typedType.Key, packageMap) + "\n" + optionalPackageOf(typedType.Value, packageMap)
	case *model.ChanType:
		return optionalPackageOf(typedType.Type, packageMap)
		// TODO:
	// case *model.FuncType:
	default:
		panic(fmt.Sprintf("TODO implement optionalPackageOf for: %v\nis type of %T\n", typedType, typedType))
	}
}

func spaceSeparatedNameFor(t model.Type, packageMap map[string]string) string {
	switch typedType := t.(type) {
	case model.PredeclaredType:
		tt := typedType.String(packageMap, "")
		if tt == "interface{}" {
			// if a predeclared type is interface
			// return a string type without curly brackets
			return "interface"
		}
		return tt
	case *model.NamedType:
		return strings.Replace((typedType.String(packageMap, "")), ".", " ", -1)
	case *model.PointerType:
		return "ptr to " + spaceSeparatedNameFor(typedType.Type, packageMap)
	case *model.ArrayType:
		if typedType.Len == -1 {
			return "slice of " + spaceSeparatedNameFor(typedType.Type, packageMap)
		} else {
			return "array of " + spaceSeparatedNameFor(typedType.Type, packageMap)
		}
	case *model.MapType:
		return "map of " + spaceSeparatedNameFor(typedType.Key, packageMap) + " to " + spaceSeparatedNameFor(typedType.Value, packageMap)
	case *model.ChanType:
		return "chan of " + spaceSeparatedNameFor(typedType.Type, packageMap)
	// TODO:
	// case *model.FuncType:
	default:
		return fmt.Sprintf("TODO implement matcher for: %v\nis type of %T\n", typedType, typedType)
	}
}

func camelcaseNameFor(t model.Type, packageMap map[string]string) string {
	return strings.Replace(strings.Title(strings.Replace(spaceSeparatedNameFor(t, packageMap), "_", " ", -1)), " ", "", -1)
}

func underscoreNameFor(t model.Type, packageMap map[string]string) string {
	return strings.ToLower(strings.Replace(spaceSeparatedNameFor(t, packageMap), " ", "_", -1))
}

func (g *generator) p(format string, args ...interface{}) *generator {
	fmt.Fprintf(&g.buf, format+"\n", args...)
	return g
}

func (g *generator) emptyLine() *generator { return g.p("") }

func (g *generator) formattedOutput() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		panic(fmt.Errorf("Failed to format generated source code: %s\n%s", err, g.buf.String()))
	}
	return src
}

func join(s []string) string { return strings.Join(s, ", ") }
