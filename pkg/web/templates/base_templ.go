// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.894
package templates

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

func BaseLayout(title string) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<!doctype html><html lang=\"en\" data-bs-theme=\"dark\"><head><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\"><title>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var2 string
		templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(title)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/web/templates/base.templ`, Line: 9, Col: 16}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, " - JS Playground</title><!-- Bootstrap CSS --><link href=\"https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css\" rel=\"stylesheet\"><!-- CodeMirror CSS --><link rel=\"stylesheet\" href=\"https://cdnjs.cloudflare.com/ajax/libs/codemirror/6.65.7/codemirror.min.css\"><link rel=\"stylesheet\" href=\"https://cdnjs.cloudflare.com/ajax/libs/codemirror/6.65.7/theme/darcula.min.css\"><!-- Custom CSS --><link rel=\"stylesheet\" href=\"/static/css/app.css\"></head><body><nav class=\"navbar navbar-expand-lg navbar-dark bg-dark\"><div class=\"container-fluid\"><a class=\"navbar-brand\" href=\"/\"><i class=\"bi bi-code-slash\"></i> JS Playground</a> <button class=\"navbar-toggler\" type=\"button\" data-bs-toggle=\"collapse\" data-bs-target=\"#navbarNav\"><span class=\"navbar-toggler-icon\"></span></button><div class=\"collapse navbar-collapse\" id=\"navbarNav\"><ul class=\"navbar-nav me-auto\"><li class=\"nav-item\"><a class=\"nav-link\" href=\"/playground\"><i class=\"bi bi-play-circle\"></i> Playground</a></li><li class=\"nav-item\"><a class=\"nav-link\" href=\"/repl\"><i class=\"bi bi-terminal\"></i> REPL</a></li><li class=\"nav-item\"><a class=\"nav-link\" href=\"/history\"><i class=\"bi bi-clock-history\"></i> History</a></li><li class=\"nav-item\"><a class=\"nav-link\" href=\"/docs\"><i class=\"bi bi-book\"></i> Docs</a></li><li class=\"nav-item\"><a class=\"nav-link\" href=\"/admin/logs\"><i class=\"bi bi-gear\"></i> Admin</a></li></ul><span class=\"navbar-text\"><i class=\"bi bi-database\"></i> Connected</span></div></div></nav><main class=\"container-fluid py-4\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templ_7745c5c3_Var1.Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "</main><!-- Bootstrap Icons --><link rel=\"stylesheet\" href=\"https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/bootstrap-icons.css\"><!-- Bootstrap JS --><script src=\"https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js\"></script><!-- CodeMirror JS --><script src=\"https://cdnjs.cloudflare.com/ajax/libs/codemirror/6.65.7/codemirror.min.js\"></script><script src=\"https://cdnjs.cloudflare.com/ajax/libs/codemirror/6.65.7/mode/javascript/javascript.min.js\"></script><script src=\"https://cdnjs.cloudflare.com/ajax/libs/codemirror/6.65.7/keymap/vim.min.js\"></script><script src=\"https://cdnjs.cloudflare.com/ajax/libs/codemirror/6.65.7/addon/edit/matchbrackets.min.js\"></script><script src=\"https://cdnjs.cloudflare.com/ajax/libs/codemirror/6.65.7/addon/edit/closebrackets.min.js\"></script><!-- Custom JS --><script src=\"/static/js/app.js\"></script></body></html>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
