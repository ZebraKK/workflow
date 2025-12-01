# workflow
这个工程是为了组织复杂任务的运行. 这些任务通常需要分多步运行。

![](https://github.com/ZebraKK/workflow/workflows/Build/badge.svg)

stage 
    包含一组step 
    这一组串行执行,则为serialTask 
    这一组并行执行,则为parallelTask 
    若有更加复杂的任务，即既有串行又有并行，那么应该使用多个task 再组合成更大的task 

step 
    最小的运行单元 


任务流可以 
    中断 
    重试 
    跳过 
    回滚 

```
设置文件, 保持LF换行

// windows
git config --global core.autocrlf true
git config --global core.eol lf

// Linux/macOS
git config --global core.autocrlf input
git config --global core.eol lf

根目录 .gitattributes 文件, 强制 Git 对所有文件使用 LF 换行
```
```
vscode 修改 使用LF换行
1. 左边齿轮 -> Settings
2. 搜索 'EOL', 选择 \n(LF)
3. 搜索 'newline',  Insert Final Newline 勾选

vscode 修改 tab转4空格
1. 左边齿轮 -> Settings
2. 搜索 'Tab', Tab Size 设置为4
3. 搜索 'insert', Insert Spaces, 勾选
4. 搜索 'Detect', Detect Indentation, 取消勾选 (避免IDE自动检测缩进)
```

```
# 递归查找所有 .go/.js/.md/.txt 文件（排除 Makefile），Tab 转 4 空格
find . -type f \( -name "*.go" -o -name "*.js" -o -name "*.md" -o -name "*.txt" \) -not -name "Makefile" -exec sed -i.bak 's/\t/    /g' {} + && find . -name "*.bak" -delete
```


github 格式参考文档：https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/quickstart-for-writing-on-github


TODO 
- 传递参数变量 
- 响应也要变量 
- 超时控制 
- job 中断/重试/跳过
- pipeline/job 的map管理

