这是一个很有意思的技术问题！Dark Reader 这类插件能做到连 `#000000` 这种硬编码内联样式都强制反转，核心靠的是 **CSS 隔离 + 浏览器扩展的 API 权限**。下面我把原理拆解清楚👇

好问题！这涉及到 Dark Reader **Dynamic（动态）模式**最核心的巧妙设计。直接看它的[源码](https://github.com/darkreader/darkreader/blob/main/src/inject/dynamic-theme/inline-style.ts)就能揭开答案👇

## 🎯 核心思路：不走寻常路

你可能会想，Dark Reader 是不是直接修改元素的 `style` 属性？比如把 `style="color: #000000"` 改成 `style="color: #ffffff"`？

**不是的。** 这样做有两大问题：
1. 你改了之后，网页自己的 JS 可能又改回来
2. 容易陷入无限循环（改→触发观察→再改→再触发）

Dark Reader 用了 **"曲线救国"** 的策略——**CSS 自定义属性 + 数据属性选择器 + !important**。

---

## 📐 详细原理拆解（三步骤）

### 第一步：扫描 + 计算

插件遍历整个 DOM，找到所有带内联样式的元素（`[style]`, `[fill]`, `[color]`, `[bgcolor]` 等），然后用颜色算法**计算出反色/暗色版本的值**。

比如页面上有个元素：
```html
<div style="color: #000000; background-color: #ffffff">
```

Dark Reader 算出来：
- `#000000`（黑字）→ `#ffffff`（白字）
- `#ffffff`（白底）→ `#000000`（黑底）

### 第二步：打标记 + 存变量（关键！）

它**不会直接改**你的 `style="color: #000000"`，而是：
- 往元素上**设置 data 属性**（标记）
- 往元素的 style 里**写入 CSS 自定义属性**（存新值）

实际效果变成了：
```html
<div style="color: #000000; 
            --darkreader-inline-color: #ffffff; 
            --darkreader-inline-bgcolor: #000000;"
     data-darkreader-inline-color
     data-darkreader-inline-bgcolor>
```

看！原来的 `color: #000000` 原封不动留着，但旁边多了自定义变量。

### 第三步：注入「终极覆盖」CSS 规则

Dark Reader 往页面里注入一条**优先级极高的样式表**：

```css
/* 用 !important 强行覆盖原始内联样式 */
[data-darkreader-inline-color] {
    color: var(--darkreader-inline-color) !important;
}
[data-darkreader-inline-bgcolor] {
    background-color: var(--darkreader-inline-bgcolor) !important;
}
[data-darkreader-inline-border] {
    border-color: var(--darkreader-inline-border) !important;
}
/* ... 以及更多属性 */
```

**为什么这招能赢过 `style="color: #000000"`？**

> 内联样式的权重确实很高，但 **`!important` 比内联样式更高**——这是 CSS 层叠规则决定的：
> - `style="color: #000000"` → 权重 1,0,0,0
> - `[data-darkreader-inline-color] { color: #fff !important; }` → 权重 0,1,1,0 + !important → **碾压**

---

## 🌓 三种模式的对比

Dark Reader 有两种模式处理这个问题：

| 模式 | 原理 | 能否覆盖内联样式 | 特点 |
|------|------|:---:|:---:|
| **Filter** | `filter: invert(1)` 整个页面反转 | ✅ | 简单粗暴，但性能差，图片也跟着反 |
| **Filter+** | SVG 滤镜，智能反转 | ✅ | 比 Filter 好一点，但仍有瑕疵 |
| **Dynamic**（默认） | **上述的 CSS 自定义属性 + !important 覆盖法** | ✅ ✅ | 最精准，性能最好，不反图片 |

---

## 💡 一个小 Demo 验证

你可以打开 DevTools 亲眼看：

1. 装 Dark Reader，打开一个网页
2. 找到任意一个带内联样式的元素（比如 `<div style="color: red">`）
3. 在 Elements 面板查看，你会发现：
   - 原始 `style` 还在（`color: red`）
   - 多了 `--darkreader-inline-color: #...` 
   - 多了 `data-darkreader-inline-color` 属性
   - Computed 面板里，最终生效的是 `color: var(--darkreader-inline-color)` ✅

---

## 🧠 总结一句话

> **Dark Reader 不是「修改」你的内联样式，而是「在它旁边存一个暗色版本的值，然后用 !important 规则强行覆盖你」**。

这就是为什么连 `style="color: #000000"` 这种硬编码也逃不掉——CSS 的 `!important` 权重规则，是浏览器原生机制赋予的"尚方宝剑" 🔑