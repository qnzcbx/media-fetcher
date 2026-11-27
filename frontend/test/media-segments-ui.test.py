import ast
from pathlib import Path
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
# 复用现有模拟 IPC 数据，不连接真实账号或下载服务。
tree = ast.parse((ROOT / "frontend/test/pgc-ui.test.py").read_text(encoding="utf-8"))
MOCK = next(ast.literal_eval(n.value) for n in tree.body if isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == "MOCK" for t in n.targets))
OUTPUT = ROOT / "notes/media-segments-screenshots"
OUTPUT.mkdir(parents=True, exist_ok=True)
with sync_playwright() as p:
 browser = p.chromium.launch(headless=True)
 for theme in ['light', 'dark']:
  page = browser.new_page(viewport={"width":960,"height":680})
  page.route('**/src/mygo.ts', lambda route: route.fulfill(status=200,content_type='text/javascript',body=MOCK))
  page.goto('http://127.0.0.1:5178')
  page.wait_for_load_state('networkidle')
  page.evaluate('(theme)=>document.documentElement.dataset.theme=theme',theme)
  page.get_by_role('textbox',name='你的链接').fill('https://www.bilibili.com/video/BV1xx411c7mD')
  page.get_by_role('button',name='下载',exact=True).click()
  page.get_by_role('button',name='高级设置').click()
  group = page.get_by_role('group',name='视频编码',exact=True)
  expect(group).to_be_visible()
  reference = group.bounding_box()
  def check_group(group):
   box = group.bounding_box()
   assert abs(box['width']-reference['width'])<1, (box,reference)
   assert abs(box['height']-reference['height'])<1, (box,reference)
   assert abs(group.locator('..').bounding_box()['height'] - 44) < 1
  check_group(group)
  page.wait_for_timeout(200)
  page.wait_for_timeout(200)
  page.screenshot(path=str(OUTPUT/f'{theme}-advanced.png'))
  page.get_by_role('button',name='关闭高级设置',exact=True).click()
  page.get_by_role('button',name='仅视频',exact=True).click()
  check_group(page.get_by_role('group',name='视频编码',exact=True))
  page.wait_for_timeout(200)
  page.screenshot(path=str(OUTPUT/f'{theme}-video.png'))
  page.get_by_role('button',name='仅音频',exact=True).click()
  check_group(page.get_by_role('group',name='输出格式',exact=True))
  page.wait_for_timeout(200)
  page.screenshot(path=str(OUTPUT/f'{theme}-audio.png'))
  page.get_by_role('button',name='关闭',exact=True).click()
  page.get_by_role('button',name='设置',exact=True).click()
  check_group(page.get_by_role('group',name='默认音频格式',exact=True))
  page.wait_for_timeout(250)
  page.wait_for_timeout(200)
  page.screenshot(path=str(OUTPUT/f'{theme}-settings.png'))
  page.close()
 browser.close()
 print('Shared media segments: four positions, light/dark, identical dimensions passed')
