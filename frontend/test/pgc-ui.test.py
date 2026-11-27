from pathlib import Path
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "notes" / "03.6B.1-visual-screenshots"
OUTPUT.mkdir(parents=True, exist_ok=True)
MOCK = r"""
let loggedIn = true;
const episodes = [1,2,3].map(id=>({id:String(id),title:id+' 测试剧集',url:'https://www.bilibili.com/bangumi/play/ep'+id+'/'}));
const base = {siteId:'bilibili',id:'ss1',title:'测试番剧',seasonTitle:'测试番剧',episodeTitle:'',coverUrl:'',isAnonymous:false,usedAccountId:'selected',episodes,videoStreams:[],audioStreams:[],images:[],sourceUrl:'https://www.bilibili.com/bangumi/play/ss1/'};
export const DesktopService = new Proxy({}, {get(_,key) {
 if(key==='extract') return async(url,account)=>{
   if(account!=='selected') throw new Error('account snapshot lost');
   if(url.includes('/ep2/')) { await new Promise(r=>setTimeout(r,100)); throw new Error('解析失败: extractor: 接口拒绝: 当前仅能试看'); }
   if(url.includes('/ep3/')) await new Promise(r=>setTimeout(r,500));
   if(url.includes('/ss')) return base;
   const result = {...base,id:'ep1',sourceUrl:url,episodeTitle:'1 测试剧集',title:'测试番剧 1 测试剧集',videoStreams:[80,64,32,16].flatMap(id=>(window.singleCodec ? ["avc1"] : ["avc1","hev1","av01"]).map(codec=>({id,quality_label:String(id),codec,width:1920,height:1080,bandwidth:100,url:"test"}))),audioStreams:[{id:30232,quality:"132Kbps 较高音质",kind:"standard",codec:"mp4a",bandwidth:132000,url:"test"},{id:30216,quality:"64Kbps 普通音质",kind:"standard",codec:"mp4a",bandwidth:64000,url:"test"},{id:30280,quality:"192Kbps",kind:"standard",codec:"mp4a",bandwidth:192000,url:"test"},{id:30251,quality:"Hi-Res 无损",kind:"hires",codec:"fLaC",bandwidth:999000,url:"test"},{id:30250,quality:"杜比音频",kind:"dolby",codec:"ec-3",bandwidth:400000,url:"test"}]};
   return url.includes('/video/') ? {...result,seasonTitle:'',episodeTitle:'',episodes:[]} : result;
 };
 if(key==='getAccounts') return async()=>loggedIn ? [{siteId:'bilibili',accountId:'selected',name:'测试账号',status:window.mockExpired?'expired':'active',isActive:true}] : [];
 if(key==='logoutAccount') return async()=> {if(window.failLogout) throw new Error('退出失败'); loggedIn=false};
 if(key==='validateDownloadDirectory') return async()=> 'C:/test';
 if(key==='startDownload') return async()=> {window.started=(window.started||0)+1};
 return async()=>[];
}});
export const events={progress:{on:()=>()=>{}}};
"""
with sync_playwright() as p:
 browser=p.chromium.launch(headless=True)
 page=browser.new_page(viewport={"width":960,"height":680})
 page.route("**/src/mygo.ts", lambda route: route.fulfill(status=200,content_type="text/javascript",body=MOCK))
 page.goto("http://127.0.0.1:5178")
 page.wait_for_load_state("networkidle")
 page.get_by_role('textbox',name='你的链接').fill('https://www.bilibili.com/bangumi/play/ss1/')
 page.get_by_role('button',name='下载',exact=True).click()
 expect(page.get_by_role('combobox',name='剧集')).to_be_visible()
 page.get_by_role('combobox',name='剧集').click()
 page.get_by_role('option',name='1 测试剧集',exact=True).click()
 expect(page.locator('.media-card')).to_have_count(4)
 page.screenshot(path=str(OUTPUT/'collapsed-mock.png'))
 before=page.locator('.resource-modal').bounding_box()
 footer=page.locator('.rmodal-footer').bounding_box()
 page.get_by_role('button',name='高级设置').click()
 expect(page.locator('.advanced-panel')).to_be_visible()
 assert page.locator('.resource-modal').bounding_box()==before
 assert page.locator('.rmodal-footer').bounding_box()==footer
 page.wait_for_timeout(250)
 page.screenshot(path=str(OUTPUT/'expanded-mock.png'))
 expect(page.get_by_role('button',name='恢复默认',exact=True)).to_be_disabled()
 page.screenshot(path=str(OUTPUT/'reset-disabled.png'))
 page.get_by_role('button',name='HEVC',exact=True).click()
 expect(page.get_by_role('button',name='HEVC',exact=True)).to_have_attribute('aria-pressed','true')
 expect(page.locator('.media-card.active')).to_have_count(1)
 page.wait_for_timeout(200)
 page.screenshot(path=str(OUTPUT/'complete-mock.png'))
 expect(page.get_by_role('button',name='恢复默认',exact=True)).to_be_enabled()
 page.screenshot(path=str(OUTPUT/'reset-normal.png'))
 page.get_by_role('button',name='恢复默认',exact=True).hover()
 page.screenshot(path=str(OUTPUT/'reset-hover.png'))
 page.get_by_role('combobox',name='音质',exact=True).click()
 expect(page.get_by_role('option',name='Hi-Res 无损',exact=True)).to_be_visible()
 assert page.get_by_role('option').all_text_contents() == ['Hi-Res 无损','杜比音频','192Kbps','132Kbps 较高音质','64Kbps 普通音质']
 trigger = page.locator('.advanced-panel .audio-select').bounding_box()
 menu = page.locator('.audio-menu').bounding_box()
 assert abs((trigger['x']+trigger['width'])-(menu['x']+menu['width'])) < 2
 assert menu['width'] < page.locator('.advanced-panel').bounding_box()['width']

 page.wait_for_timeout(200)
 page.screenshot(path=str(OUTPUT/'audio-menu-mock.png'))
 page.get_by_role('option',name='Hi-Res 无损',exact=True).click()
 expect(page.locator('.advanced-panel')).to_be_visible()
 page.get_by_role('button',name='恢复默认',exact=True).click()
 page.get_by_role('button',name='关闭高级设置',exact=True).click()
 page.get_by_role('button',name='仅视频',exact=True).click()
 page.wait_for_timeout(200)
 page.screenshot(path=str(OUTPUT/'video-mock.png'))
 page.get_by_role('button',name='仅音频',exact=True).click()
 page.locator('.media-card').filter(has_text='Hi-Res 无损').click()
 page.get_by_role('button',name='原始',exact=True).click()
 expect(page.get_by_role('button',name='下载原始音频',exact=True)).to_be_enabled()
 page.wait_for_timeout(200)
 page.screenshot(path=str(OUTPUT/'audio-original-mock.png'))
 page.get_by_role('button',name='完整视频',exact=True).click()
 for size in [(960,680),(1280,800),(720,600)]:
  page.set_viewport_size({"width":size[0],"height":size[1]})
  for theme in ['light','dark']:
   page.evaluate('(theme)=>document.documentElement.dataset.theme=theme',theme)
   page.wait_for_timeout(200)
   page.screenshot(path=str(OUTPUT/f'{size[0]}-{theme}.png'))
   page.get_by_role('button',name='高级设置').click()
   expect(page.locator('.advanced-panel')).to_be_visible()
   page.screenshot(path=str(OUTPUT/f'{size[0]}-{theme}-expanded.png'))
   page.get_by_role('combobox',name='音质',exact=True).click()
   expect(page.get_by_role('option',name='Hi-Res 无损',exact=True)).to_be_visible()
   page.screenshot(path=str(OUTPUT/f'{size[0]}-{theme}-menu.png'))
   page.keyboard.press('Escape')
   expect(page.locator('.advanced-panel')).to_be_visible()
   page.keyboard.press('Escape')
   expect(page.locator('.advanced-panel')).to_have_count(0)
   rect=page.locator('.media-card').first.bounding_box()
   footer=page.locator('.rmodal-footer').bounding_box()
   assert rect['y']+rect['height']<=footer['y'], '首行卡片被遮挡'
 page.get_by_role('combobox',name='剧集').click()
 page.get_by_role('option',name='2 测试剧集',exact=True).click()
 expect(page.locator('.media-card')).to_have_count(0)
 expect(page.get_by_role('button',name='下载 MP4')).to_be_disabled()
 page.locator('.download-btn').evaluate("button=>button.dispatchEvent(new MouseEvent('click',{bubbles:true}))")
 assert page.evaluate('window.started||0')==0
 expect(page.get_by_role('alert')).to_have_text('当前剧集仅提供试看资源。')
 expect(page.get_by_role('combobox',name='剧集')).to_contain_text('2 测试剧集')
 page.wait_for_timeout(200)
 page.screenshot(path=str(OUTPUT/'episode-error.png'))
 page.get_by_role('button',name='重试',exact=True).click()
 expect(page.get_by_role('alert')).to_have_text('当前剧集仅提供试看资源。')
 page.get_by_role('combobox',name='剧集').click()
 page.get_by_role('option',name='1 测试剧集',exact=True).click()
 expect(page.locator('.media-card')).to_have_count(4)
 page.get_by_role('combobox',name='剧集').click()
 page.get_by_role('option',name='3 测试剧集',exact=True).click()
 page.get_by_role('button',name='关闭',exact=True).click()
 expect(page.locator('.resource-modal')).to_have_count(0)
 page.wait_for_timeout(650)
 expect(page.locator('.resource-modal')).to_have_count(0)
 expect(page.get_by_role('alert')).to_have_count(0)
 assert page.evaluate('window.started||0')==0
 page.get_by_role('button',name='凭证管理',exact=True).click()
 page.get_by_role('button',name='退出此账号',exact=True).click()
 expect(page.get_by_role('dialog')).to_contain_text('退出后将清除本地登录凭证。')
 assert page.locator('.dialog-title').evaluate("e=>getComputedStyle(e).fontSize") == '18px'
 assert page.locator('.logout-dialog .delete-hint').evaluate("e=>getComputedStyle(e).marginLeft") == '0px'
 page.wait_for_timeout(200)
 page.screenshot(path=str(OUTPUT/'logout-confirmation.png'))
 page.get_by_role('button',name='取消',exact=True).click()
 expect(page.get_by_role('button',name='退出此账号',exact=True)).to_be_visible()
 page.evaluate('window.failLogout=true')
 page.get_by_role('button',name='退出此账号',exact=True).click()
 page.get_by_role('button',name='退出',exact=True).click()
 expect(page.get_by_role('alert')).to_have_text('退出失败')
 page.evaluate('window.failLogout=false')
 page.get_by_role('button',name='退出',exact=True).click()
 expect(page.get_by_role('button',name='登录 Bilibili',exact=True)).to_be_visible()
 page.add_init_script('window.mockExpired=true')
 page.reload()
 page.wait_for_load_state('networkidle')
 page.get_by_role('button',name='凭证管理',exact=True).click()
 expect(page.get_by_text('测试账号 (已过期)',exact=True)).to_be_visible()
 page.get_by_role('button',name='退出此账号',exact=True).click()
 page.get_by_role('button',name='退出',exact=True).click()
 expect(page.get_by_role('button',name='登录 Bilibili',exact=True)).to_be_visible()
 page.add_init_script('window.mockExpired=false')
 page.reload()
 page.wait_for_load_state('networkidle')
 page.get_by_role('textbox',name='你的链接').fill('https://www.bilibili.com/video/BV1xx411c7mD')
 page.get_by_role('button',name='下载',exact=True).click()
 expect(page.locator('.media-card')).to_have_count(4)
 page.get_by_role('button',name='高级设置').click()
 page.screenshot(path=str(OUTPUT/'ugc-expanded-mock.png'))
 page.get_by_role('button',name='关闭高级设置').click()
 page.get_by_role('button',name='关闭',exact=True).click()
 page.evaluate('window.singleCodec=true')
 page.get_by_role('button',name='下载',exact=True).click()
 page.get_by_role('button',name='高级设置').click()
 expect(page.get_by_role('button',name='AVC',exact=True)).to_be_visible()
 expect(page.get_by_role('button',name='HEVC',exact=True)).to_have_count(0)
 browser.close()
 print('PGC UI state and layout checks passed')
