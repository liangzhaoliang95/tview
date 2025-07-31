package main

import (
	"bytes"
	"fmt"
	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/quick"
	"github.com/alecthomas/chroma/styles"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// 打开系统默认编辑器
func openSystemEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// fallback 到常见编辑器
		for _, candidate := range []string{"nano", "vim", "vi", "code"} {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("未设置 $EDITOR，且未找到可用编辑器（如 vim/nano）")
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// 定义 chroma => tview 颜色格式映射
func styleToTviewStyle(s chroma.StyleEntry) string {
	if s.Colour.IsSet() {
		return s.Colour.String() // 输出成标准颜色名
	}
	return ""
}

// 自定义 formatter：将 token 转为 [color]... 的格式
type TviewFormatter struct{}

func (f *TviewFormatter) Format(w io.Writer, style *chroma.Style, iterator chroma.Iterator) error {
	for token := iterator(); token != chroma.EOF; token = iterator() {
		entry := style.Get(token.Type)
		color := styleToTviewStyle(entry)

		if color != "" {
			fmt.Fprintf(w, "[%s]%s[white]", color, token.Value)
		} else {
			fmt.Fprintf(w, "%s", token.Value)
		}
	}
	return nil
}

// 返回一个 formatter 实例
func NewTviewFormatter() chroma.Formatter {
	return &TviewFormatter{}
}

// 使用自定义 TviewFormatter 渲染语法高亮为 tview 可识别格式
func renderHighlightedForTview(content, path string) string {
	var out bytes.Buffer

	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return content // 返回原始内容
	}

	formatter := NewTviewFormatter()
	style := styles.Get("monokai") // 可改为 dracula/native/solarized
	if style == nil {
		style = styles.Fallback
	}

	err = formatter.Format(&out, style, iterator)
	if err != nil {
		return content
	}

	return out.String()
}

// 检测语言类型
func detectLangByExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".sh":
		return "bash"
	case ".toml":
		return "toml"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	default:
		return "" // 让 chroma 自动猜测
	}
}

// 渲染高亮内容
func renderHighlighted(content, path string) string {
	var buf bytes.Buffer
	lang := detectLangByExt(path)
	err := quick.Highlight(&buf, content, lang, "terminal256", "monokai")
	if err != nil {
		return content // 回退原始内容
	}
	return buf.String()
}

func main() {
	rootDir := "/Users/liang" // 用当前路径，可以替换为"/Users/liang"等绝对路径

	// 文件树节点
	rootNode := tview.NewTreeNode(rootDir).
		SetColor(tcell.ColorRed).
		SetReference(rootDir)

	tree := tview.NewTreeView().
		SetRoot(rootNode).
		SetCurrentNode(rootNode)

	// 文件预览区域
	preview := tview.NewTextView()
	preview.
		SetDynamicColors(true).
		SetWordWrap(true).
		SetBorder(true).
		SetTitle("预览")

	// 编辑区域（进入编辑模式时显示）
	editor := tview.NewTextView().SetChangedFunc(func() {

	})
	editor.
		SetDynamicColors(true).
		SetBorder(true).
		SetTitle("编辑中...(按 Esc 退出)")

	// 页面布局
	flex := tview.NewFlex().
		AddItem(tree, 40, 1, true).
		AddItem(preview, 0, 2, false)

	app := tview.NewApplication()

	// 添加节点的辅助函数
	addChildren := func(node *tview.TreeNode, path string) {
		files, err := os.ReadDir(path)
		if err != nil {
			// 显示错误信息
			preview.SetText(fmt.Sprintf("[red]读取目录失败: %v", err))
			return
		}
		for _, file := range files {
			fullPath := filepath.Join(path, file.Name())
			child := tview.NewTreeNode(file.Name()).
				SetReference(fullPath).
				SetSelectable(true)
			if file.IsDir() {
				child.SetColor(tcell.ColorGreen)
			}
			node.AddChild(child)
		}
	}

	// 初始展开根目录
	addChildren(rootNode, rootDir)
	tree.SetChangedFunc(func(node *tview.TreeNode) {
		// 当节点变化时，更新预览内容
		ref := node.GetReference()
		if ref == nil {
			preview.SetText("[red]请选择一个文件或目录")
			return
		}
		path := ref.(string)
		info, err := os.Stat(path)
		if err != nil {
			preview.SetText(fmt.Sprintf("[red]读取失败: %v", err))
			return
		}

		if info.IsDir() {
			// 不处理
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				preview.SetText(fmt.Sprintf("[red]无法读取文件: %v", err))
			} else {
				preview.SetTitle("预览: " + filepath.Base(path))
				highlighted := renderHighlightedForTview(string(content), path)
				preview.SetText(highlighted)
			}
			//app.SetFocus(preview)
		}
	})
	// 节点选择时事件
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			preview.SetText("[red]请选择一个文件或目录")
			return
		}
		path := ref.(string)

		info, err := os.Stat(path)
		if err != nil {
			preview.SetText(fmt.Sprintf("[red]读取失败: %v", err))
			return
		}

		if info.IsDir() {
			if len(node.GetChildren()) == 0 {
				addChildren(node, path)
				node.SetExpanded(true) // ✅ 第一次加载子目录 -> 展开它
			} else {
				node.SetExpanded(!node.IsExpanded())
			}

		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				preview.SetText(fmt.Sprintf("[red]无法读取文件: %v", err))
			} else {
				preview.SetTitle("预览: " + filepath.Base(path))
				highlighted := renderHighlightedForTview(string(content), path)
				preview.SetText(highlighted)
			}
			//app.SetFocus(preview)
		}
	})

	// 键盘事件捕获逻辑
	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'e':
			node := tree.GetCurrentNode()
			if node == nil {
				return nil
			}
			ref := node.GetReference()
			if ref == nil {
				return nil
			}
			path := ref.(string)

			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				return nil
			}

			//🟡 暂停 tview UI 进入外部编辑器（阻塞直到编辑完成）
			app.Suspend(func() {
				err := openSystemEditor(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[编辑器打开失败] %v\n", err)
					fmt.Println("按回车继续...")
					fmt.Scanln()
				}
			})

			// 🟢 回到 TUI，自动刷新预览内容
			data, err := os.ReadFile(path)
			if err != nil {
				preview.SetText("[red]文件读取失败")
			} else {
				preview.SetTitle("预览: " + filepath.Base(path))
				preview.SetText(string(data))
				// 👇自动切焦点到右侧预览区域
				app.SetFocus(preview)
			}
		}
		return event
	})

	// 编辑器按 Esc 退出编辑
	editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			app.SetRoot(flex, true).SetFocus(tree)
			return nil
		}
		return event
	})

	var inPreviewFocus = false // 当前焦点标记

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			if inPreviewFocus {
				app.SetFocus(tree)
			} else {
				app.SetFocus(preview)
			}
			inPreviewFocus = !inPreviewFocus
			return nil
		}
		return event
	})

	// 启动程序
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
