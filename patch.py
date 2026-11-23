import re
with open('core/extractor/lofter/lofter.go', 'r', encoding='utf-8') as f:
    content = f.read()

old_block = """					if u.RawQuery != "" {
						cleanU := *u
						cleanU.RawQuery = ""
						primaryURL = cleanU.String()
						backupUrls = append(backupUrls, uStr)
					}"""

new_block = """					if u.RawQuery != "" && u.Query().Has("imageView") {
						cleanU := *u
						q := cleanU.Query()
						q.Del("imageView")
						q.Del("thumbnail")
						q.Del("quality")
						q.Del("stripmeta")
						q.Del("type")
						cleanU.RawQuery = q.Encode()
						primaryURL = cleanU.String()
						backupUrls = append(backupUrls, uStr)
					}"""

content = content.replace(old_block, new_block)

with open('core/extractor/lofter/lofter.go', 'w', encoding='utf-8') as f:
    f.write(content)
