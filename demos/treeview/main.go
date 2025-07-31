package main

import (
	"bytes"
	"fmt"
	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/quick"
	"github.com/alecthomas/chroma/styles"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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

	// 节点选择时事件
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
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
		}
	})

	// 键盘事件捕获逻辑
	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'e': // 进入编辑模式
			node := tree.GetCurrentNode()
			ref := node.GetReference()
			if ref == nil {
				return nil
			}
			path := ref.(string)
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				return nil // 仅支持编辑文件
			}
			content, err := ioutil.ReadFile(path)
			if err != nil {
				preview.SetText("无法读取文件用于编辑: " + err.Error())
				return nil
			}
			editor.SetText(string(content))
			app.SetRoot(editor, true).SetFocus(editor)
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

	// 启动程序
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
