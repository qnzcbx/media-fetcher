with open("cmd/media-fetcher-cli/main.go", "r", encoding="utf-8") as f:
    content = f.read()

bad = """					if err != nil {
							fmt.Printf("失败: %v\n", err)"""
bad2 = "							fmt.Printf(\"失败: %v\n\", err)"

if bad in content:
    content = content.replace(bad, """					if err != nil {\n\t\t\t\t\t\tfmt.Printf("失败: %v\n", err)""")
else:
    # Just fix the newline issue
    content = content.replace("失败: %v\n\", err)", "失败: %v\n\", err)")

with open("cmd/media-fetcher-cli/main.go", "w", encoding="utf-8") as f:
    f.write(content)
