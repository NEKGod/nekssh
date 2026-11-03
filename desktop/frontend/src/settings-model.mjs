export const settingsKey='nekssh.desktop.settings.v1';
export const terminalFontStack=family=>family==='monospace'?'monospace':`"${family}", Consolas, monospace`;
export const defaultSettings=Object.freeze({
  accent:'mint',density:'comfortable',fontFamily:'Consolas',fontSize:14,lineHeight:1,
  cursorStyle:'block',cursorBlink:true,scrollback:10000,showSidebar:true,
  statsInterval:15,portsInterval:10,debug:false
});
const pick=(value,allowed,fallback)=>allowed.includes(value)?value:fallback;
const integer=(value,min,max,fallback)=>Number.isInteger(value)&&value>=min&&value<=max?value:fallback;
export function normalizeSettings(value){
  const v=value&&typeof value==='object'?value:{};
  return {
    accent:pick(v.accent,['mint','blue','violet'],defaultSettings.accent),
    density:pick(v.density,['comfortable','compact'],defaultSettings.density),
    fontFamily:pick(v.fontFamily,['Consolas','Cascadia Mono','JetBrains Mono','monospace'],defaultSettings.fontFamily),
    fontSize:integer(v.fontSize,11,24,defaultSettings.fontSize),
    lineHeight:pick(v.lineHeight,[1,1.2,1.4],defaultSettings.lineHeight),
    cursorStyle:pick(v.cursorStyle,['block','bar','underline'],defaultSettings.cursorStyle),
    cursorBlink:typeof v.cursorBlink==='boolean'?v.cursorBlink:defaultSettings.cursorBlink,
    scrollback:integer(v.scrollback,1000,100000,defaultSettings.scrollback),
    showSidebar:typeof v.showSidebar==='boolean'?v.showSidebar:defaultSettings.showSidebar,
    statsInterval:pick(v.statsInterval,[5,10,15,30,60],defaultSettings.statsInterval),
    portsInterval:pick(v.portsInterval,[5,10,15,30,60],defaultSettings.portsInterval),
    debug:typeof v.debug==='boolean'?v.debug:defaultSettings.debug
  };
}
export function createSettingsStore(storage){
  let initial;
  try{
    const saved=storage?.getItem(settingsKey);
    initial=saved?JSON.parse(saved):{
      showSidebar:storage?.getItem('nekssh.desktop.sidebar.collapsed')!=='1',
      debug:storage?.getItem('nekssh.desktop.debug')==='1'
    };
  }catch{}
  let current=normalizeSettings(initial);
  return {
    get(){return {...current}},
    save(value){
      const next=normalizeSettings(value);
      // A failed disk write must not masquerade as successfully saved settings.
      if(!storage) throw Error('本机存储不可用，设置未保存');
      storage.setItem(settingsKey,JSON.stringify(next));
      current=next;return {...current};
    }
  };
}
