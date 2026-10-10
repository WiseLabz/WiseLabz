package api_test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/go-chi/chi/v5"
	"golang.org/x/tools/go/packages"
)

// elevationFixtures is keyed by operation so runtime router discovery and
// end-to-end requests share one inventory without a second source map.
type elevationFixture struct {
	source string
	action string
}

var elevationFixtures = map[string]elevationFixture{
	"POST /auth/api-keys":                  {"RequireElevation", "apiKey.create"},
	"PUT /auth/config":                     {"RequireElevation", "authConfig.update"},
	"PUT /auth/providers/{p}/enabled":      {"RequireElevationForTarget", "authProvider.toggle"},
	"POST /me/mfa/totp":                    {"RequireElevationUnlessEnrollOnly", "mfa.manage"},
	"POST /me/mfa/webauthn/register/begin": {"RequireElevationUnlessEnrollOnly", "mfa.manage"},
	"POST /me/mfa/recovery-codes":          {"RequireElevation", "mfa.manage"},
	"DELETE /me/mfa/factors/{p}":           {"RequireElevation", "mfa.manage"},
	"POST /users":                          {"RequireElevation", "user.create"},
	"PATCH /users/{p}":                     {"RequireElevationForTarget", "user.update"},
	"DELETE /users/{p}":                    {"RequireElevationForTarget", "user.delete"},
	"POST /users/{p}/reset-password":       {"RequireElevationForTarget", "user.resetPassword"},
	"POST /users/{p}/reset-mfa":            {"RequireElevationForTarget", "user.resetMfa"},
	"DELETE /connectors/{p}":               {"RequireElevation", "connector.delete"},
	"POST /connectors/bulk-restart":        {"RequireElevation", "connector.bulkRestart"},
	"POST /connectors/{p}/restart":         {"ValidateElevationHeader", "connector.restart"},
	"POST /connectors/{p}/start":           {"ValidateElevationHeader", "connector.start"},
	"POST /connectors/{p}/stop":            {"ValidateElevationHeader", "connector.stop"},
	"POST /connectors/{p}/actions/{p}":     {"ValidateElevationHeaderFor", "connector.action"},
	"POST /connectors/{p}/config-push":     {"ValidateElevationHeader", "connector.configPush"},
	"POST /connectors":                     {"ValidateElevationHeaderFor", "connector.recipeActions"},
	"PUT /connectors/{p}":                  {"ValidateElevationHeaderFor", "connector.recipeActions"},
	"POST /docs/import/pull":               {"RequireElevation", "docs.import.pull"},
	"DELETE /templates/{p}":                {"RequireElevation", "template.delete"},
	"POST /discovery/scan":                 {"RequireElevation", "discovery.scan"},
	"POST /runbooks/{p}/run":               {"ValidateElevationHeaderFor", "runbook.run"},
	"POST /runbooks/{p}/steps/{p}/execute": {"ValidateElevationHeader", "runbook.run"},
	"POST /runbook-runs/{p}/approve":       {"ValidateElevationHeaderFor", "runbook.approve"},
	"POST /runbook-runs/{p}/resume":        {"ValidateElevationHeaderFor", "runbook.run"},
}

func TestElevationGuardMetadataMatchesRouter(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	routes, ok := app.Router.(chi.Routes)
	if !ok {
		t.Fatalf("router is %T, want chi.Routes", app.Router)
	}

	got := map[string]map[string]string{"/api": {}, "/api/v1": {}}
	err := chi.Walk(routes, func(
		method, route string,
		handler http.Handler,
		middlewares ...func(http.Handler) http.Handler,
	) error {
		prefix := ""
		for _, candidate := range []string{"/api/v1", "/api"} {
			if strings.HasPrefix(route, candidate+"/") {
				prefix = candidate
				break
			}
		}
		if prefix == "" {
			return nil
		}
		source := ""
		if marker, ok := handler.(interface{ ElevationSource() string }); ok {
			source = marker.ElevationSource()
		}
		for _, middleware := range middlewares {
			wrapped := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			if marker, ok := wrapped.(interface{ ElevationSource() string }); ok {
				source = marker.ElevationSource()
			}
		}
		if source == "" {
			return nil
		}
		route = strings.TrimPrefix(route, prefix)
		route = strings.TrimSuffix(route, "/")
		operation := strings.ToUpper(method) + " " + normalizeParams(route)
		if prior, exists := got[prefix][operation]; exists && prior != source {
			return fmt.Errorf("%s has conflicting elevation sources %q and %q", operation, prior, source)
		}
		got[prefix][operation] = source
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}

	for prefix, operations := range got {
		for _, issue := range elevationInventoryIssues(operations, elevationFixtures) {
			t.Errorf("%s %s", prefix, issue)
		}
	}
}

func elevationInventoryIssues(operations map[string]string, fixtures map[string]elevationFixture) []string {
	issues := []string{}
	for operation, fixture := range fixtures {
		if operations[operation] != fixture.source {
			issues = append(issues, fmt.Sprintf(
				"%s elevation source = %q, want %q",
				operation, operations[operation], fixture.source,
			))
		}
	}
	for operation := range operations {
		if _, ok := fixtures[operation]; !ok {
			issues = append(issues, fmt.Sprintf("guarded operation lacks response fixture: %s", operation))
		}
	}
	sort.Strings(issues)
	return issues
}

func TestElevationInventoryRejectsMissingFixture(t *testing.T) {
	issues := elevationInventoryIssues(
		map[string]string{"POST /new-guarded-operation": "RequireNewGuard"},
		map[string]elevationFixture{},
	)
	if len(issues) != 1 || !strings.Contains(issues[0], "guarded operation lacks response fixture") {
		t.Fatalf("new guarded operation did not report its missing response fixture: %v", issues)
	}
}

type routeGuardRegistration struct {
	packagePath string
	handler     *types.Func
	literal     *ast.FuncLit
	info        *types.Info
	source      string
}

type expressionRef struct {
	expr ast.Expr
	info *types.Info
}

// TestElevationMetadataMatchesReachableGuards uses go/packages type
// information to follow registered method handlers through local helpers.
// Conditional branches and helper calls count as reachable; unknown handler
// expressions and unannotated guarded handlers fail closed.
func TestElevationMetadataMatchesReachableGuards(t *testing.T) {
	backendDir := filepath.Dir(filepath.Dir(filepath.Dir(sourceFile(t))))
	config := &packages.Config{
		Dir: backendDir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}
	loaded, err := packages.Load(config, "./internal/api/...", "./internal/auth")
	if err != nil {
		t.Fatalf("load API packages: %v", err)
	}
	if packages.PrintErrors(loaded) > 0 {
		t.Fatal("type loading reported errors")
	}
	decls := map[*types.Func][]*ast.FuncDecl{}
	infos := map[*types.Package]*types.Info{}
	varInitializers := map[types.Object][]expressionRef{}
	registrations := []routeGuardRegistration{}
	for _, pkg := range loaded {
		infos[pkg.Types] = pkg.TypesInfo
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.ValueSpec:
					for index, name := range value.Names {
						if index < len(value.Values) {
							if object := pkg.TypesInfo.Defs[name]; object != nil {
								varInitializers[object] = append(
									varInitializers[object],
									expressionRef{expr: value.Values[index], info: pkg.TypesInfo},
								)
							}
						}
					}
				case *ast.AssignStmt:
					for index, lhs := range value.Lhs {
						rhsIndex := index
						if len(value.Rhs) == 1 {
							rhsIndex = 0
						}
						if rhsIndex >= len(value.Rhs) {
							continue
						}
						ident, ok := lhs.(*ast.Ident)
						if !ok {
							continue
						}
						object := pkg.TypesInfo.Defs[ident]
						if object == nil {
							object = pkg.TypesInfo.Uses[ident]
						}
						if object != nil {
							varInitializers[object] = append(
								varInitializers[object],
								expressionRef{expr: value.Rhs[rhsIndex], info: pkg.TypesInfo},
							)
						}
					}
				case *ast.RangeStmt:
					if values, ok := value.X.(*ast.CompositeLit); ok {
						if ident, ok := value.Value.(*ast.Ident); ok {
							object := pkg.TypesInfo.Defs[ident]
							if object == nil {
								object = pkg.TypesInfo.Uses[ident]
							}
							for _, elt := range values.Elts {
								if object != nil {
									varInitializers[object] = append(varInitializers[object], expressionRef{expr: elt, info: pkg.TypesInfo})
								}
							}
						}
					}
				}
				return true
			})
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee := calledFunc(call.Fun, pkg.TypesInfo)
				if !isRouterRegistration(callee) {
					return true
				}
				if len(call.Args) == 0 {
					t.Errorf("%s: route registration %s has no handler", pkg.Fset.Position(call.Pos()), callee.Name())
					return true
				}
				handler := call.Args[len(call.Args)-1]
				source := ""
				if marker, ok := handler.(*ast.CallExpr); ok && selectorName(marker.Fun) == "withElevationSource" {
					if len(marker.Args) != 2 {
						t.Errorf("%s: unsupported withElevationSource arguments", pkg.Fset.Position(marker.Pos()))
						return true
					}
					lit, ok := marker.Args[0].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Errorf("%s: elevation source must be a string literal", pkg.Fset.Position(marker.Pos()))
						return true
					}
					var unquoteErr error
					source, unquoteErr = strconv.Unquote(lit.Value)
					if unquoteErr != nil || source == "" {
						t.Errorf("%s: unsupported elevation source %q", pkg.Fset.Position(marker.Pos()), lit.Value)
						return true
					}
					handler = marker.Args[1]
				}
				var fn *types.Func
				var literal *ast.FuncLit
				switch handlerExpr := handler.(type) {
				case *ast.FuncLit:
					literal = handlerExpr
				default:
					fn = calledFunc(handler, pkg.TypesInfo)
				}
				if fn == nil && literal == nil {
					t.Errorf("%s: unsupported registered handler expression %T", pkg.Fset.Position(handler.Pos()), handler)
					return true
				}
				registrations = append(registrations, routeGuardRegistration{
					packagePath: pkg.PkgPath,
					handler:     fn,
					literal:     literal,
					info:        pkg.TypesInfo,
					source:      source,
				})
				return true
			})
			for _, node := range file.Decls {
				decl, ok := node.(*ast.FuncDecl)
				if !ok || decl.Body == nil {
					continue
				}
				fn, _ := pkg.TypesInfo.Defs[decl.Name].(*types.Func)
				if fn != nil {
					decls[fn] = append(decls[fn], decl)
				}
			}
		}
	}

	if len(registrations) == 0 {
		t.Fatal("no route handlers discovered")
	}
	for _, pkg := range loaded {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				fn := calledFunc(call.Fun, pkg.TypesInfo)
				if fn == nil || len(decls[fn]) == 0 {
					return true
				}
				signature, ok := fn.Type().(*types.Signature)
				if !ok {
					return true
				}
				for index, arg := range call.Args {
					if index < signature.Params().Len() {
						parameter := signature.Params().At(index)
						varInitializers[parameter] = append(varInitializers[parameter], expressionRef{expr: arg, info: pkg.TypesInfo})
					}
				}
				return true
			})
		}
	}
	guardSources := discoverElevationGuardSources(decls, infos)
	validateMiddlewareGuardFactories(t, loaded, decls, infos, guardSources, varInitializers)
	for _, registration := range registrations {
		reachable, unsupported := reachableElevationSources(
			registration.handler,
			registration.literal,
			registration.info,
			decls,
			infos,
			guardSources,
			varInitializers,
		)
		handlerName := "anonymous handler"
		if registration.handler != nil {
			handlerName = registration.handler.Name()
		}
		for _, call := range unsupported {
			t.Errorf("%s.%s contains an unsupported indirect call: %s", registration.packagePath, handlerName, call)
		}
		if issue := elevationMetadataIssue(registration.source, reachable, guardSources); issue != "" {
			t.Errorf("%s.%s %s (reachable guards %v)", registration.packagePath, handlerName, issue, keys(reachable))
		}
	}
}

func elevationMetadataIssue(source string, reachable, guardSources map[string]bool) string {
	if len(reachable) == 0 {
		if source != "" {
			return fmt.Sprintf("has stale elevation source metadata %q", source)
		}
		return ""
	}
	if source == "" {
		return "reaches elevation guards without handler metadata"
	}
	if !guardSources[source] {
		return fmt.Sprintf("metadata names %q, which does not reach the elevation sink", source)
	}
	if !reachable[source] {
		return fmt.Sprintf("metadata says %q, but that guard is not reachable", source)
	}
	return ""
}

func validateMiddlewareGuardFactories(
	t *testing.T,
	loaded []*packages.Package,
	decls map[*types.Func][]*ast.FuncDecl,
	infos map[*types.Package]*types.Info,
	guardSources map[string]bool,
	initializers map[types.Object][]expressionRef,
) {
	knownFixtureSources := map[string]bool{}
	for _, fixture := range elevationFixtures {
		knownFixtureSources[fixture.source] = true
	}
	for _, pkg := range loaded {
		if !strings.HasSuffix(pkg.PkgPath, "/internal/api") &&
			!strings.HasPrefix(pkg.PkgPath, "github.com/WiseLabz/wiselabz/internal/api/") {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isRouterMiddlewareRegistration(calledFunc(call.Fun, pkg.TypesInfo)) {
					return true
				}
				for _, arg := range call.Args {
					candidates := middlewareFactoryCandidates(arg, pkg.TypesInfo, initializers)
					for _, candidate := range candidates {
						sources, unsupported := reachableElevationSources(
							candidate, nil, pkg.TypesInfo, decls, infos, guardSources, initializers,
						)
						for source := range sources {
							for _, call := range unsupported {
								t.Errorf("middleware guard factory %s contains unsupported indirect call: %s", source, call)
							}
							if !knownFixtureSources[source] {
								t.Errorf("middleware guard factory %s is absent from the elevation fixture inventory", source)
							}
							if !middlewareFactoryHasInspectableSource(candidate, decls, infos, map[*types.Func]bool{}) {
								t.Errorf(
									"middleware guard factory %s reaches the elevation sink but its returned "+
										"handler has no inspectable source metadata",
									source,
								)
							}
						}
					}
				}
				return true
			})
		}
	}
}

func isRouterMiddlewareRegistration(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "github.com/go-chi/chi/v5" {
		return false
	}
	return fn.Name() == "Use" || fn.Name() == "With"
}

func middlewareFactoryCandidates(
	expr ast.Expr,
	info *types.Info,
	initializers map[types.Object][]expressionRef,
) []*types.Func {
	seen := map[*types.Func]bool{}
	var collect func(ast.Expr)
	collect = func(expr ast.Expr) {
		switch value := expr.(type) {
		case *ast.CallExpr:
			if fn := calledFunc(value.Fun, info); fn != nil {
				seen[fn] = true
			}
			for _, arg := range value.Args {
				collect(arg)
			}
		case *ast.Ident:
			if fn := calledFunc(value, info); fn != nil {
				seen[fn] = true
				return
			}
			if object := info.Uses[value]; object != nil {
				for _, initializer := range initializers[object] {
					collect(initializer.expr)
				}
			}
		case *ast.SelectorExpr:
			if fn := calledFunc(value, info); fn != nil {
				seen[fn] = true
			}
		}
	}
	collect(expr)
	result := make([]*types.Func, 0, len(seen))
	for fn := range seen {
		result = append(result, fn)
	}
	return result
}

func middlewareFactoryHasInspectableSource(
	fn *types.Func,
	decls map[*types.Func][]*ast.FuncDecl,
	infos map[*types.Package]*types.Info,
	seen map[*types.Func]bool,
) bool {
	if fn == nil || seen[fn] {
		return false
	}
	seen[fn] = true
	for _, decl := range decls[fn] {
		if decl.Body == nil {
			continue
		}
		returns := returnExpressions(decl.Body)
		if len(returns) > 0 && allReturnedHandlersInspectable(returns, infos[fn.Pkg()], decls, infos, seen) {
			return true
		}
	}
	return false
}

func allReturnedHandlersInspectable(
	expressions []ast.Expr,
	info *types.Info,
	decls map[*types.Func][]*ast.FuncDecl,
	infos map[*types.Package]*types.Info,
	seen map[*types.Func]bool,
) bool {
	if len(expressions) == 0 {
		return false
	}
	for _, expression := range expressions {
		if !returnedHandlerInspectable(expression, info, decls, infos, seen) {
			return false
		}
	}
	return true
}

func returnedHandlerInspectable(
	expression ast.Expr,
	info *types.Info,
	decls map[*types.Func][]*ast.FuncDecl,
	infos map[*types.Package]*types.Info,
	seen map[*types.Func]bool,
) bool {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return returnedHandlerInspectable(value.X, info, decls, infos, seen)
	case *ast.CompositeLit:
		return inspectableElevationHandler(info.Types[value].Type)
	case *ast.FuncLit:
		return allReturnedHandlersInspectable(returnExpressions(value.Body), info, decls, infos, seen)
	case *ast.CallExpr:
		called := calledFunc(value.Fun, info)
		if called == nil || len(decls[called]) == 0 {
			return inspectableElevationHandler(info.Types[value].Type)
		}
		return middlewareFactoryHasInspectableSource(called, decls, infos, seen)
	default:
		return inspectableElevationHandler(info.Types[expression].Type)
	}
}

func returnExpressions(body *ast.BlockStmt) []ast.Expr {
	var expressions []ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncLit:
			return false // Nested closures are checked only when returned.
		case *ast.ReturnStmt:
			expressions = append(expressions, value.Results...)
		}
		return true
	})
	return expressions
}

func inspectableElevationHandler(handlerType types.Type) bool {
	if handlerType == nil {
		return false
	}
	methodSet := types.NewMethodSet(handlerType)
	source := methodSet.Lookup(nil, "ElevationSource")
	serve := methodSet.Lookup(nil, "ServeHTTP")
	if source == nil || serve == nil {
		return false
	}
	sourceSig, sourceOK := source.Type().(*types.Signature)
	serveSig, serveOK := serve.Type().(*types.Signature)
	if !sourceOK || !serveOK || sourceSig.Params().Len() != 0 || sourceSig.Results().Len() != 1 ||
		!types.Identical(sourceSig.Results().At(0).Type(), types.Typ[types.String]) {
		return false
	}
	return serveSig.Params().Len() == 2 && serveSig.Results().Len() == 0 &&
		isHTTPType(serveSig.Params().At(0).Type(), "ResponseWriter", false) &&
		isHTTPType(serveSig.Params().At(1).Type(), "Request", true)
}

func isHTTPType(got types.Type, name string, pointer bool) bool {
	if pointer {
		ptr, ok := got.(*types.Pointer)
		if !ok {
			return false
		}
		got = ptr.Elem()
	}
	named, ok := got.(*types.Named)
	return ok && named.Obj().Name() == name && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "net/http"
}

func sourceFile(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return file
}

func selectorName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.Ident:
		return value.Name
	default:
		return ""
	}
}

func calledFunc(expr ast.Expr, info *types.Info) *types.Func {
	switch value := expr.(type) {
	case *ast.SelectorExpr:
		if selection := info.Selections[value]; selection != nil {
			fn, _ := selection.Obj().(*types.Func)
			return fn
		}
		fn, _ := info.Uses[value.Sel].(*types.Func)
		return fn
	case *ast.Ident:
		fn, _ := info.Uses[value].(*types.Func)
		return fn
	case *ast.IndexExpr:
		return calledFunc(value.X, info)
	case *ast.IndexListExpr:
		return calledFunc(value.X, info)
	default:
		return nil
	}
}

func unsupportedCall(expr ast.Expr, info *types.Info) bool {
	if info.Types[expr].IsType() {
		return false
	}
	switch value := expr.(type) {
	case *ast.FuncLit:
		return false
	case *ast.Ident:
		if _, ok := info.Uses[value].(*types.Builtin); ok || info.Types[value].IsType() {
			return false
		}
		if calledFunc(value, info) != nil {
			return false
		}
		if object, ok := info.Uses[value].(*types.Var); ok {
			_, functionValue := object.Type().Underlying().(*types.Signature)
			return functionValue
		}
		return false
	case *ast.SelectorExpr:
		if info.Types[value].IsType() {
			return false
		}
		if calledFunc(value, info) != nil {
			return false
		}
		identity := callbackIdentity(value, info)
		return !knownCallbackLeaf[identity]
	case *ast.IndexExpr:
		return unsupportedCall(value.X, info)
	case *ast.IndexListExpr:
		return unsupportedCall(value.X, info)
	default:
		return calledFunc(expr, info) == nil
	}
}

// These function-valued collaborators are injected by constructors and do
// not carry API/auth handlers: runtime settings, connector execution, and
// discovery's OS interface lookup. Other indirect selector calls fail closed.
var knownCallbackLeaf = map[string]bool{
	"github.com/WiseLabz/wiselabz/internal/api/runbooks.Handler.settings":            true,
	"github.com/WiseLabz/wiselabz/internal/api/connectors.preparedLifecycleOp.apply": true,
	"github.com/WiseLabz/wiselabz/internal/api/discovery.Handler.interfaceAddrs":     true,
	"github.com/WiseLabz/wiselabz/internal/auth.Service.settings":                    true,
}

func TestElevationAnalyzerCallbackLeavesAreExplicit(t *testing.T) {
	if knownCallbackLeaf["github.com/WiseLabz/wiselabz/internal/api/new.Handler.newElevationGuard"] {
		t.Fatal("unknown callback must not be silently accepted")
	}
}

func callbackIdentity(selector *ast.SelectorExpr, info *types.Info) string {
	selection := info.Selections[selector]
	if selection == nil {
		return exprName(selector.X) + "." + selector.Sel.Name
	}
	object, ok := selection.Obj().(*types.Var)
	if !ok || object.Pkg() == nil {
		return ""
	}
	receiver := selection.Recv()
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = pointer.Elem()
	}
	named, ok := receiver.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + object.Name()
}

func TestElevationAnalyzerFindsNewAuthGuardThroughHelper(t *testing.T) {
	authPkg, authFile, authInfo := typecheckElevationSource(t, "example/internal/auth", `package auth
import "net/http"
type Service struct{}
type elevationGuardHandler struct{}
func (elevationGuardHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
func (elevationGuardHandler) ElevationSource() string { return "RequireNewGuard" }
type lookalike struct{}
func (lookalike) ServeHTTP(http.ResponseWriter, *http.Request) {}
func (s *Service) ConsumeElevation() {}
func (s *Service) ValidateElevationHeader() { s.ConsumeElevation() }
func (s *Service) RequireNewGuard() func(http.Handler) http.Handler {
	s.ValidateElevationHeader()
	return func(http.Handler) http.Handler { return elevationGuardHandler{} }
}
func (s *Service) RequirePlainGuard() func(http.Handler) http.Handler {
	s.ConsumeElevation()
	return func(http.Handler) http.Handler { return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}) }
}
func (s *Service) RequireDecoyGuard() func(http.Handler) http.Handler {
	s.ConsumeElevation()
	_ = elevationGuardHandler{}
	return func(http.Handler) http.Handler { return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}) }
}
func (s *Service) RequireWrongShapeGuard() func(http.Handler) http.Handler {
	s.ConsumeElevation()
	return func(http.Handler) http.Handler { return lookalike{} }
}
`, sourceImporter{})
	apiPkg, apiFile, apiInfo := typecheckElevationSource(t, "example/internal/api", `package api
import "example/internal/auth"
func guardedHelper(s *auth.Service) { s.RequireNewGuard() }
func route(s *auth.Service) { guardedHelper(s) }
`, sourceImporter{packages: map[string]*types.Package{"example/internal/auth": authPkg}})

	decls := make(map[*types.Func][]*ast.FuncDecl)
	for _, entry := range []struct {
		pkg  *types.Package
		file *ast.File
		info *types.Info
	}{{authPkg, authFile, authInfo}, {apiPkg, apiFile, apiInfo}} {
		for _, node := range entry.file.Decls {
			decl, ok := node.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			if fn, ok := entry.info.Defs[decl.Name].(*types.Func); ok {
				decls[fn] = append(decls[fn], decl)
			}
		}
	}
	infos := map[*types.Package]*types.Info{authPkg: authInfo, apiPkg: apiInfo}
	guardSources := discoverElevationGuardSources(decls, infos)
	if !guardSources["RequireNewGuard"] {
		t.Fatal("new auth guard calling ValidateElevationHeader was not discovered through ConsumeElevation")
	}
	method := func(name string) *types.Func {
		for fn := range decls {
			if fn.Pkg() == authPkg && fn.Name() == name {
				return fn
			}
		}
		return nil
	}
	if !middlewareFactoryHasInspectableSource(method("RequireNewGuard"), decls, infos, map[*types.Func]bool{}) {
		t.Fatal("new guard returning an inspectable elevation handler was not recognized")
	}
	if middlewareFactoryHasInspectableSource(method("RequirePlainGuard"), decls, infos, map[*types.Func]bool{}) {
		t.Fatal("new guard factory without inspectable handler metadata was accepted")
	}
	if middlewareFactoryHasInspectableSource(method("RequireDecoyGuard"), decls, infos, map[*types.Func]bool{}) {
		t.Fatal("unused elevation metadata literal was mistaken for a returned handler")
	}
	if middlewareFactoryHasInspectableSource(method("RequireWrongShapeGuard"), decls, infos, map[*types.Func]bool{}) {
		t.Fatal("returned handler without the complete inspectable interface was accepted")
	}
	route := apiInfo.Defs[apiFile.Decls[2].(*ast.FuncDecl).Name].(*types.Func)
	reachable, unsupported := reachableElevationSources(
		route, nil, apiInfo, decls, infos, guardSources, map[types.Object][]expressionRef{},
	)
	if len(unsupported) != 0 {
		t.Fatalf("unexpected unsupported calls in synthetic route: %v", unsupported)
	}
	if issue := elevationMetadataIssue("", reachable, guardSources); issue == "" {
		t.Fatal("an unannotated route reusing a guarded helper was accepted")
	}
	if issue := elevationMetadataIssue("RequireNewGuard", reachable, guardSources); issue != "" {
		t.Fatalf("new guard metadata was rejected: %s", issue)
	}
}

func TestElevationAnalyzerRejectsUnresolvedFunctionVariable(t *testing.T) {
	pkg, file, info := typecheckElevationSource(t, "example/internal/api", `package api
var indirect func()
func route() { indirect() }
`, nil)
	_ = pkg
	call := file.Decls[1].(*ast.FuncDecl).Body.List[0].(*ast.ExprStmt).X.(*ast.CallExpr)
	if !unsupportedCall(call.Fun, info) {
		t.Fatal("unresolved function variable was silently accepted")
	}
}

type sourceImporter struct{ packages map[string]*types.Package }

func (i sourceImporter) Import(path string) (*types.Package, error) {
	pkg := i.packages[path]
	if pkg != nil {
		return pkg, nil
	}
	return importer.Default().Import(path)
}

func typecheckElevationSource(
	t *testing.T,
	path, source string,
	imp types.Importer,
) (*types.Package, *ast.File, *types.Info) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path+".go", source, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	config := types.Config{Importer: imp}
	pkg, err := config.Check(path, fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("typecheck synthetic source: %v", err)
	}
	return pkg, file, info
}

func isRouterRegistration(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "github.com/go-chi/chi/v5" {
		return false
	}
	switch fn.Name() {
	case "Get", "Post", "Put", "Patch", "Delete", "Method", "Handle", "HandleFunc":
		return true
	default:
		return false
	}
}

func discoverElevationGuardSources(
	decls map[*types.Func][]*ast.FuncDecl,
	infos map[*types.Package]*types.Info,
) map[string]bool {
	guardSources := map[string]bool{}
	for fn := range decls {
		if fn.Pkg() == nil || !strings.HasSuffix(fn.Pkg().Path(), "/internal/auth") {
			continue
		}
		if functionReachesElevationSink(fn, decls, infos, map[*types.Func]bool{}) {
			guardSources[fn.Name()] = true
		}
	}
	return guardSources
}

func functionReachesElevationSink(
	fn *types.Func,
	decls map[*types.Func][]*ast.FuncDecl,
	infos map[*types.Package]*types.Info,
	seen map[*types.Func]bool,
) bool {
	if isElevationSink(fn) {
		return true
	}
	if fn == nil || seen[fn] {
		return false
	}
	seen[fn] = true
	for _, decl := range decls[fn] {
		if decl.Body == nil {
			continue
		}
		found := false
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := calledFunc(call.Fun, infos[fn.Pkg()])
			if callee == nil {
				return true
			}
			// ConsumeElevation is the enforcement sink. Tracing to it means
			// new auth helpers become visible without maintaining a guard-name list.
			if isElevationSink(callee) {
				found = true
				return false
			}
			if callee.Pkg() != nil && strings.HasSuffix(callee.Pkg().Path(), "/internal/auth") &&
				functionReachesElevationSink(callee, decls, infos, seen) {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

func isElevationSink(fn *types.Func) bool {
	if fn == nil || fn.Name() != "ConsumeElevation" || fn.Pkg() == nil ||
		!strings.HasSuffix(fn.Pkg().Path(), "/internal/auth") {
		return false
	}
	signature, ok := fn.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}
	recv := signature.Recv().Type()
	if pointer, ok := recv.(*types.Pointer); ok {
		recv = pointer.Elem()
	}
	named, ok := recv.(*types.Named)
	return ok && named.Obj().Name() == "Service"
}

func reachableElevationSources(
	start *types.Func,
	literal *ast.FuncLit,
	info *types.Info,
	decls map[*types.Func][]*ast.FuncDecl,
	infos map[*types.Package]*types.Info,
	guardSources map[string]bool,
	varInitializers map[types.Object][]expressionRef,
) (map[string]bool, []string) {
	seen := map[*types.Func]bool{}
	found := map[string]bool{}
	unsupported := []string{}
	var visitBody func(ast.Node, *types.Info)
	var visit func(*types.Func)
	seenValues := map[types.Object]bool{}
	var visitFunctionValue func(types.Object, string, token.Pos)
	var visitFunctionExpr func(ast.Expr, *types.Info, string, token.Pos)
	visitFunctionValue = func(object types.Object, name string, pos token.Pos) {
		if object == nil || seenValues[object] {
			return
		}
		seenValues[object] = true
		initializers, ok := varInitializers[object]
		if !ok || len(initializers) == 0 {
			unsupported = append(unsupported, fmt.Sprintf("unresolved function variable %s at token position %d", name, pos))
			return
		}
		for _, initializer := range initializers {
			visitFunctionExpr(initializer.expr, initializer.info, name, initializer.expr.Pos())
		}
	}
	visitFunctionExpr = func(expr ast.Expr, exprInfo *types.Info, name string, pos token.Pos) {
		if fn := calledFunc(expr, exprInfo); fn != nil {
			visit(fn)
			return
		}
		switch value := expr.(type) {
		case *ast.FuncLit:
			visitBody(value.Body, exprInfo)
		case *ast.Ident:
			visitFunctionValue(exprInfo.Uses[value], name, pos)
		case *ast.CallExpr:
			visit(calledFunc(value.Fun, exprInfo))
		default:
			if unsupportedCall(expr, exprInfo) {
				unsupported = append(unsupported, fmt.Sprintf(
					"unresolved function variable %s initializer %s at token position %d",
					name, exprName(expr), pos,
				))
			}
		}
	}
	visit = func(fn *types.Func) {
		if fn == nil || seen[fn] {
			return
		}
		seen[fn] = true
		if fn.Pkg() != nil && strings.HasSuffix(fn.Pkg().Path(), "/internal/auth") && guardSources[fn.Name()] {
			found[fn.Name()] = true
			return
		}
		for _, decl := range decls[fn] {
			if decl.Body != nil {
				visitBody(decl.Body, infos[fn.Pkg()])
			}
		}
	}
	visitBody = func(body ast.Node, bodyInfo *types.Info) {
		ast.Inspect(body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if called := calledFunc(call.Fun, bodyInfo); called != nil {
				visit(called)
			} else if ident, ok := call.Fun.(*ast.Ident); ok {
				object := bodyInfo.Uses[ident]
				if _, isVar := object.(*types.Var); isVar {
					visitFunctionValue(object, ident.Name, call.Pos())
				} else if unsupportedCall(call.Fun, bodyInfo) {
					unsupported = append(unsupported, fmt.Sprintf("unsupported call %s at token position %d", ident.Name, call.Pos()))
				}
			} else if unsupportedCall(call.Fun, bodyInfo) {
				description := exprName(call.Fun)
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
					description += " (" + callbackIdentity(selector, bodyInfo) + ")"
				}
				unsupported = append(unsupported, fmt.Sprintf("%s at token position %d", description, call.Pos()))
			}
			return true
		})
	}
	visit(start)
	if literal != nil {
		visitBody(literal.Body, info)
	}
	return found, unique(unsupported)
}

func exprName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return exprName(value.X) + "." + value.Sel.Name
	case *ast.IndexExpr:
		return exprName(value.X)
	case *ast.IndexListExpr:
		return exprName(value.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func keys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func TestElevationFailuresMatchOpenAPI(t *testing.T) {
	for operation := range elevationFixtures {
		operation := operation
		t.Run(operation, func(t *testing.T) {
			app := newTestApp(t)
			stepUp(app, true)
			_, token := app.user(t, "operator")
			for _, prefix := range []string{"/api", "/api/v1"} {
				t.Run(prefix, func(t *testing.T) {
					req := elevationFixtureRequest(t, app, prefix, operation, token)
					missing := app.serve(req)
					assertElevationFailure(t, req, missing, http.StatusBadRequest, "elevation_required")

					rejectedReq := elevationFixtureRequest(t, app, prefix, operation, token)
					rejectedReq.Header.Set("X-Elevation-Token", "not-a-valid-elevation-token")
					rejected := app.serve(rejectedReq)
					assertElevationFailure(t, rejectedReq, rejected, http.StatusUnauthorized, "unauthorized")
				})
			}
		})
	}
}

func assertElevationFailure(t *testing.T, req *http.Request, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Errorf("decode %s %s response: %v", req.Method, req.URL.Path, err)
	}
	if rec.Code != status || body.Code != code {
		t.Errorf("%s %s -> %d %s, want %d %s", req.Method, req.URL.Path, rec.Code, rec.Body.String(), status, code)
	}
	response := &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(strings.NewReader(rec.Body.String())),
	}
	apitest.AssertMatchesSpec(t, req, response)
}
