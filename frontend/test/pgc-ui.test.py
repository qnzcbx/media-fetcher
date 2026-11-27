from pathlib import Path
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "notes" / "03.6A.1-screenshots"
OUTPUT.mkdir(parents=True, exist_ok=True)
MOCK = r"""
const episodes = [1,2,3].map(id=>({id:String(id),title:id+' 测试剧集',url:'https://www.bilibili.com/bangumi/play/ep'+id+'/'}));
const base = {siteId:'bilibili',id:'ss1',title:'测试番剧',seasonTitle:'测试番剧',episodeTitle:'',coverUrl:'',isAnonymous:false,usedAccountId:'selected',episodes,videoStreams:[],audioStreams:[],images:[],sourceUrl:'https://www.bilibili.com/bangumi/play/ss1/'};
export const DesktopService = new Proxy({}, {get(_,key) {
 if(key==='extract') return async(url,account)=>{
   if(account!=='selected') throw new Error('account snapshot lost');
   if(url.includes('/ep2/')) { await new Promise(r=>setTimeout(r,100)); throw new Error('解析失败: extractor: 接口拒绝: 当前仅能试看'); }
   if(url.includes('/ep3/')) await new Promise(r=>setTimeout(r,500));
   if(url.includes('/ss')) return base;
   return {...base,id:'ep1',sourceUrl:url,episodeTitle:'1 测试剧集',title:'测试番剧 1 测试剧集',videoStreams:[80,64,32].map(id=>({id,quality_label:String(id),codec:'avc1',width:1920,height:1080,bandwidth:100,url:'test'})),audioStreams:[{id:30280,quality:'192Kbps',codec:'mp4a',bandwidth:192000,url:'test'}]};
 };
 if(key==='getAccounts') return async()=>[{siteId:'bilibili',accountId:'selected',status:'active',isActive:true}];
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
 expect(page.locator('.media-card')).to_have_count(3)
 for size in [(960,680),(1280,900)]:
  page.set_viewport_size({"width":size[0],"height":size[1]})
  for theme in ['light','dark']:
   page.evaluate('(theme)=>document.documentElement.dataset.theme=theme',theme)
   page.screenshot(path=str(OUTPUT/f'{size[0]}-{theme}.png'))
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
 page.screenshot(path=str(OUTPUT/'episode-error.png'))
 page.get_by_role('button',name='重试',exact=True).click()
 expect(page.get_by_role('alert')).to_have_text('当前剧集仅提供试看资源。')
 page.get_by_role('combobox',name='剧集').click()
 page.get_by_role('option',name='1 测试剧集',exact=True).click()
 expect(page.locator('.media-card')).to_have_count(3)
 page.get_by_role('combobox',name='剧集').click()
 page.get_by_role('option',name='3 测试剧集',exact=True).click()
 page.get_by_role('button',name='关闭',exact=True).click()
 expect(page.locator('.resource-modal')).to_have_count(0)
 page.wait_for_timeout(650)
 expect(page.locator('.resource-modal')).to_have_count(0)
 expect(page.get_by_role('alert')).to_have_count(0)
 assert page.evaluate('window.started||0')==0
 browser.close()
 print('PGC UI state and layout checks passed')
