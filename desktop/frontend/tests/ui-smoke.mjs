// Read-only UI fixtures: no real SSH connection, credentials, or remote writes.
// Run: node tests/ui-smoke.mjs [artifact-directory]
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {access,mkdir,mkdtemp,readFile,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {createServer} from 'vite';

const browser=process.env.BROWSER_PATH||'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
await access(browser);
const artifacts=process.argv[2]?resolve(process.argv[2]):await mkdtemp(join(tmpdir(),'nekssh-ui-'));
await mkdir(artifacts,{recursive:true});
const fixture={
  hosts:[{name:'开发服务器',username:'root',host:'192.168.1.9',port:22},{name:'生产环境',username:'deploy',host:'10.0.0.12',port:22}],
  files:['src','docs','public','package.json','README.md','server.go','config.yaml','.gitignore'].map((Name,i)=>({Name,Path:'/srv/nekssh/'+Name,Directory:i<3,Size:2048*(i+1),Modified:'2026-10-31T09:32:00Z',Mode:i<3?'drwxr-xr-x':'-rw-r--r--',Owner:'root',Group:'root'})),
  processes:['node','sshd','nginx','postgres','codex','systemd','redis-server'].map((Command,i)=>({PID:4210+i*23,CPU:12.4/(i+1),Memory:4.8/(i+1),Command,User:'root',Ports:i===0?'8888':''})),
  ports:[22,80,443,8888].map(Port=>({Protocol:'tcp',Address:'0.0.0.0',Port,PID:4210,Process:Port===8888?'node':'nginx',User:'root',Command:'service --port '+Port})),
  tunnels:[80,8888].map(RemotePort=>({ID:'127.0.0.1:'+RemotePort,RemoteAddress:'127.0.0.1:'+RemotePort,LocalAddress:'127.0.0.1:'+RemotePort,RemotePort,CreatedAt:RemotePort})),
  stats:{Host:'dev-server',CPUPercent:23.6,MemoryUsed:3.2*2**30,MemoryTotal:8*2**30,DiskUsed:42*2**30,DiskTotal:128*2**30,SwapUsed:0,SwapTotal:2*2**30},
  partitions:[{Mount:'/',Device:'/dev/sda1',Used:42*2**30,Total:128*2**30,Percent:33},{Mount:'/data',Device:'/dev/sdb1',Used:156*2**30,Total:512*2**30,Percent:30}]
};
const terminalText='\x1b[38;2;121;215;190mWelcome to your remote workspace\x1b[0m\r\n\r\nroot@dev-server:/srv/nekssh$ npm run dev\r\n\r\n  \x1b[32mVITE\x1b[0m  ready in 284 ms\r\n\r\n  Local:   http://localhost:8888/\r\n  Network: http://192.168.1.9:8888/\r\n\r\n  \x1b[90mSSH connected · SFTP ready · Reverse bridge online\x1b[0m\r\n\r\nroot@dev-server:/srv/nekssh$ ';
const server=await createServer({
  root:resolve(import.meta.dirname,'..'),appType:'custom',
  plugins:[{name:'ui-fixture-hook',transform(code,id){
    if(id.replaceAll('\\','/').endsWith('/src/main.js')) return code+`\nwindow.__ui={async seed(){sessionId='ui-session';sshSessions.set(sessionId,{saved:window.fixture.hosts[0],connected:true,path:'/srv/nekssh',commandBuffer:'',commandHistory:['npm run dev','git status','go test ./...'],outputChunks:[]});$('#welcome').hidden=true;$('#workspace').hidden=false;renderSessionTabs();setStatus('已连接','#79d7be');showReflect('127.0.0.1:18765','');await loadFiles('/srv/nekssh');startStatsRefresh('开发服务器');await new Promise(r=>term.write(${JSON.stringify(terminalText)},r));scheduleTerminalFit()},term,refit:refitTerminal,get editor(){return editorView}};`;
  }}],server:{host:'127.0.0.1',port:0,hmr:false,watch:{ignored:['**/wailsjs/**']}}
});
server.middlewares.use('/ui-probe',(req,res)=>{
  res.setHeader('Content-Type','text/html; charset=utf-8');
  res.end(`<meta charset="utf-8"><div id="app"></div><script>
    window.fixture=${JSON.stringify(fixture)};window.uiCalls=[];window.uiBrowserOpens=[];window.uiProjectEntries={};
    if(!sessionStorage.getItem('uiFixtureReady')){localStorage.clear();localStorage.setItem('nekssh.desktop.hosts',JSON.stringify(fixture.hosts));sessionStorage.setItem('uiFixtureReady','1')}
    window.uiIntervals=new Map();const nativeInterval=window.setInterval.bind(window),nativeClear=window.clearInterval.bind(window);
    window.setInterval=(fn,delay,...args)=>{const id=nativeInterval(fn,delay,...args);uiIntervals.set(id,delay);return id};window.clearInterval=id=>{uiIntervals.delete(id);nativeClear(id)};
    window.go={main:{App:new Proxy({},{get:(_,key)=>async(...args)=>{
      uiCalls.push({key,args});
      if(key==='StartPortTunnel'){
        if(window.uiFailTunnel)throw Error('服务器端口 10020 无法连接');
        const remote=args[2]+':'+args[3];
        let tunnel=fixture.tunnels.find(t=>t.RemoteAddress===remote);
        if(!tunnel){tunnel={ID:'127.0.0.1:54321',LocalAddress:'127.0.0.1:54321',RemoteAddress:remote,RemotePort:args[3]};fixture.tunnels.push(tunnel)}
        return tunnel;
      }
      if(key==='CreateProject'){
        const path=args[1]+'/'+args[2];
        uiProjectEntries[path]=[{Name:'main.go',Path:path+'/main.go',Directory:false,Size:64,Modified:'2026-10-31',Mode:'-rw-r--r--',Owner:'root',Group:'root'}];
        return {Path:path,Template:args[3],Files:['main.go']};
      }
      if(key==='ListFiles')return uiProjectEntries[args[1]]||fixture.files;
      if(key==='ReadRemoteFile')return ${JSON.stringify('package main\n\nfunc main() {}\n')};
      return ({Version:'UI Preview',SetDebug:'',ListFiles:fixture.files,ServerProcesses:fixture.processes,ServerStats:fixture.stats,ServerPartitions:fixture.partitions,ListeningPorts:fixture.ports,PortTunnels:fixture.tunnels,CommandHistory:[]})[key]??[];
    }})}};window.runtime=new Proxy({},{get:(_,key)=>(...args)=>{if(key==='BrowserOpenURL'){if(window.uiFailBrowserOpen)throw Error('mock browser unavailable');uiBrowserOpens.push(args[0])}}});
    </script><script type="module" src="/src/main.js"></script>`);
});
await server.listen();
const profile=await mkdtemp(join(tmpdir(),'nekssh-ui-browser-'));
const child=spawn(browser,['--headless=new','--disable-gpu','--no-first-run','--remote-debugging-port=0','--user-data-dir='+profile,'about:blank'],{windowsHide:true,stdio:'ignore'});
let ws;
try{
  let port;
  for(let i=0;i<100;i++){
    try{port=(await readFile(join(profile,'DevToolsActivePort'),'utf8')).split('\n')[0];break}catch{await new Promise(r=>setTimeout(r,100))}
  }
  assert.ok(port,'browser did not start');
  const pages=await(await fetch('http://127.0.0.1:'+port+'/json/list')).json();
  ws=new WebSocket(pages.find(page=>page.type==='page').webSocketDebuggerUrl);
  await new Promise(resolve=>ws.addEventListener('open',resolve,{once:true}));
  let sequence=0;const pending=new Map();
  ws.addEventListener('message',event=>{const message=JSON.parse(event.data);if(message.id){pending.get(message.id)?.(message);pending.delete(message.id)}});
  const call=(method,params={})=>new Promise((resolve,reject)=>{
    const id=++sequence;const timer=setTimeout(()=>{pending.delete(id);reject(Error('CDP timeout: '+method))},20000);
    pending.set(id,message=>{clearTimeout(timer);message.error?reject(Error(JSON.stringify(message.error))):resolve(message.result)});
    ws.send(JSON.stringify({id,method,params}));
  });
  const evaluate=async expression=>{
    const result=await call('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true});
    if(result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails));
    return result.result?.value;
  };
  const size=(width,height)=>call('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false});
  const screenshot=async name=>{await evaluate('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');const result=await call('Page.captureScreenshot',{format:'png'});await writeFile(join(artifacts,name+'.png'),Buffer.from(result.data,'base64'))};
  await size(1280,820);
  await call('Page.navigate',{url:server.resolvedUrls.local[0]+'ui-probe'});
  let ready=false;
  for(let i=0;i<100;i++){if(await evaluate('Boolean(window.__ui)')){ready=true;break}await new Promise(r=>setTimeout(r,100))}
  assert.ok(ready,'app did not initialize');
  assert.equal(await evaluate(`getComputedStyle(document.querySelector('#workspace')).display`),'none');
  await screenshot('01-welcome');
  await evaluate(`document.querySelector('#welcomeConnect').click()`);
  assert.equal(await evaluate(`document.querySelector('#connectDialog').open`),true);
  await screenshot('02-connect');
  await evaluate(`document.querySelector('#connectDialog').close();window.__ui.seed()`);
  await new Promise(r=>setTimeout(r,150));
  assert.equal(await evaluate(`document.querySelectorAll('#fileList .file:not(.head)').length`),fixture.files.length+1);
  assert.equal(await evaluate(`document.querySelector('#promptButton').disabled`),false);
  await screenshot('03-workspace');
  await evaluate(`document.querySelector('#portsButton').click()`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`document.querySelectorAll('.port-tunnel').length`),2);
  await screenshot('04-ports');
  await evaluate(`document.querySelector('#portsDialog [data-close]').click();document.querySelector('#promptButton').click();document.querySelector('[data-prompt-skill="client-browser"]').click();document.querySelector('#terminalPromptInput').value='用客户机 Chrome 打开 localhost:8888，检查页面是否正常。';document.querySelector('#terminalPromptInput').dispatchEvent(new Event('input'))`);
  assert.equal(await evaluate(`document.querySelector('#terminalPromptSubmit').disabled`),false);
  await screenshot('05-prompt');
  await evaluate(`document.querySelector('#terminalPromptForm').requestSubmit()`);
  const input=await evaluate(`window.uiCalls.filter(call=>call.key==='Input').at(-1)`);
  assert.equal(input.args[0],'ui-session');assert.ok(input.args[1].includes('CDP'));assert.doesNotMatch(input.args[1],/[\r\n]/);
  // Make sure fitting after layout changes preserves scrollback position.
  await evaluate(`new Promise(r=>__ui.term.write(Array.from({length:160},(_,i)=>'output '+i+'\\r\\n').join(''),r))`);
  await evaluate(`__ui.term.scrollToLine(20);document.querySelector('#sidebarToggle').click()`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`__ui.term.buffer.active.viewportY`),20);
  const handle=await evaluate(`(()=>{const r=document.querySelector('#fileResizeHandle').getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2,height:document.querySelector('#files').getBoundingClientRect().height}})()`);
  await call('Input.dispatchMouseEvent',{type:'mousePressed',x:handle.x,y:handle.y,button:'left',clickCount:1});
  await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:handle.x,y:handle.y-35,button:'left',buttons:1});
  await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:handle.x,y:handle.y-35,button:'left',clickCount:1});
  await new Promise(r=>setTimeout(r,100));
  assert.ok(await evaluate(`document.querySelector('#files').getBoundingClientRect().height`)>handle.height);
  assert.equal(await evaluate(`document.body.classList.contains('resizing-files')`),false);
  // Row count changes can shift the top line; dragging must not snap to the
  // live output or disable subsequent scrollback navigation.
  const afterDrag=await evaluate(`__ui.term.buffer.active.viewportY`);
  assert.ok(afterDrag<await evaluate(`__ui.term.buffer.active.baseY`));
  await evaluate(`__ui.term.scrollLines(1)`);
  assert.equal(await evaluate(`__ui.term.buffer.active.viewportY`),afterDrag+1);
  await evaluate(`__ui.term.scrollLines(-1);document.querySelector('#fileList .file:not(.head)').click()`);
  assert.equal(await evaluate(`document.querySelectorAll('#fileList .file.selected').length`),1);
  assert.equal(await evaluate(`document.querySelector('#sidebarToggle').getAttribute('aria-expanded')`),'false');
  await evaluate(`document.querySelector('#sidebarToggle').click()`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`__ui.term.buffer.active.viewportY`),afterDrag);
  // Restore a clean terminal for small-window screenshots.
  await evaluate(`__ui.term.reset();new Promise(r=>__ui.term.write(${JSON.stringify(terminalText)},r))`);
  for(const [width,height] of [[1024,768],[900,600]]){
    await size(width,height);await new Promise(r=>setTimeout(r,120));
    const overflow=await evaluate(`({document:document.documentElement.scrollWidth>innerWidth,toolbar:document.querySelector('header').scrollWidth>document.querySelector('header').clientWidth,workspace:document.querySelector('#workspace').getBoundingClientRect().bottom>document.querySelector('.workspace-statusbar').getBoundingClientRect().top+1})`);
    assert.deepEqual(overflow,{document:false,toolbar:false,workspace:false});
    await screenshot('06-workspace-'+width);
  }
  // All primary flyouts remain inside the application at its minimum width.
  for(const [button,panel] of [['hostMenuButton','hostMenu'],['transferButton','transferPanel'],['commandHistoryButton','commandHistoryPanel']]){
    await evaluate(`document.querySelector('#${button}').click()`);
    assert.equal(await evaluate(`(()=>{const r=document.querySelector('#${panel}').getBoundingClientRect();return r.left>=0&&r.right<=innerWidth})()`),true);
    await evaluate(`document.querySelector('#${button}').click()`);
  }
  await size(1280,820);
  await evaluate(`document.querySelector('#settingsButton').click()`);
  assert.equal(await evaluate(`document.querySelector('#settingsDialog').open`),true);
  await screenshot('07-settings-appearance');
  await evaluate(`(()=>{const f=document.querySelector('#settingsForm');f.elements.namedItem('accent').value='blue';f.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('#settingsDialog [data-settings-close]').click()})()`);
  assert.equal(await evaluate(`document.body.dataset.accent`),'mint');
  await evaluate(`document.querySelector('#settingsButton').click()`);
  assert.equal(await evaluate(`document.querySelector('#settingsForm').elements.namedItem('accent').value`),'mint');
  await evaluate(`(()=>{const f=document.querySelector('#settingsForm');f.elements.namedItem('accent').value='blue';f.elements.namedItem('density').value='compact';document.querySelector('[data-settings-tab="terminal"]').click();f.elements.namedItem('fontSize').value='0';document.querySelector('[data-settings-tab="appearance"]').click();f.requestSubmit()})()`);
  assert.equal(await evaluate(`document.querySelector('[data-settings-tab="terminal"]').getAttribute('aria-selected')`),'true');
  assert.equal(await evaluate(`__ui.term.options.fontSize`),14);
  await evaluate(`(()=>{const f=document.querySelector('#settingsForm');f.elements.namedItem('fontSize').value='18';f.elements.namedItem('lineHeight').value='1.2';f.elements.namedItem('cursorStyle').value='bar';f.elements.namedItem('cursorBlink').checked=false;f.elements.namedItem('scrollback').value='5000';f.dispatchEvent(new Event('input',{bubbles:true}))})()`);
  await screenshot('08-settings-terminal');
  await evaluate(`(()=>{const f=document.querySelector('#settingsForm');document.querySelector('[data-settings-tab="workspace"]').click();f.elements.namedItem('statsInterval').value='30';f.elements.namedItem('portsInterval').value='15';f.elements.namedItem('showSidebar').checked=false;f.requestSubmit()})()`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`document.querySelector('#settingsDialog').open`),false);
  assert.deepEqual(await evaluate(`({font:__ui.term.options.fontSize,line:__ui.term.options.lineHeight,cursor:__ui.term.options.cursorStyle,blink:__ui.term.options.cursorBlink,history:__ui.term.options.scrollback,accent:document.body.dataset.accent,density:document.body.dataset.density,sidebar:document.body.classList.contains('sidebar-collapsed')})`),{font:18,line:1.2,cursor:'bar',blink:false,history:5000,accent:'blue',density:'compact',sidebar:true});
  assert.ok((await evaluate(`[...uiIntervals.values()]`)).includes(30000));
  await evaluate(`document.querySelector('#portsButton').click()`);
  assert.ok((await evaluate(`[...uiIntervals.values()]`)).includes(15000));
  await evaluate(`document.querySelector('#portsDialog [data-close]').click();document.querySelector('#settingsButton').click();document.querySelector('#settingsReset').click();document.querySelector('#settingsDialog [data-settings-close]').click()`);
  assert.equal(await evaluate(`__ui.term.options.fontSize`),18);
  // Reload the real entry module and confirm saved settings apply at startup.
  await call('Page.reload');
  for(let i=0;i<100;i++){if(await evaluate(`Boolean(window.__ui&&document.body.dataset.settingsReady)`))break;await new Promise(r=>setTimeout(r,100))}
  assert.equal(await evaluate(`__ui.term.options.fontSize`),18);
  assert.equal(await evaluate(`document.body.dataset.accent`),'blue');
  await evaluate(`document.querySelector('#settingsButton').click();document.querySelector('#settingsReset').click();document.querySelector('#settingsForm').requestSubmit()`);
  await new Promise(r=>setTimeout(r,50));
  assert.equal(await evaluate(`__ui.term.options.fontSize`),14);
  assert.equal(await evaluate(`JSON.parse(localStorage.getItem('nekssh.desktop.hosts')).length`),2);
  await evaluate(`__ui.seed()`);
  await evaluate(`(()=>{const row=document.querySelector('#fileList .directory:not(.parent-directory)');row.click();document.querySelector('#projectNew').click()})()`);
  assert.equal(await evaluate(`document.querySelector('#projectForm').elements.namedItem('parent').value`),'/srv/nekssh/src');
  await evaluate(`document.querySelector('#projectDialog [data-project-close]').click();document.querySelector('#fileList .directory:not(.parent-directory)').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,clientX:450,clientY:400}));document.querySelector('[data-action="newProject"]').click()`);
  assert.equal(await evaluate(`document.querySelector('#projectDialog').open`),true);
  assert.equal(await evaluate(`document.querySelector('#projectForm').elements.namedItem('parent').value`),'/srv/nekssh/src');
  await evaluate(`(()=>{const input=document.querySelector('#projectForm').elements.namedItem('projectName');input.value='my-app';input.dispatchEvent(new Event('input'));document.querySelector('[data-project-template="go"]').click()})()`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`document.querySelector('#projectSubmit').disabled`),false);
  assert.equal(await evaluate(`Boolean(document.querySelector('#projectForm select[name="listener"]'))`),false);
  await screenshot('09-create-project');
  await evaluate(`window.uiFailTunnel=true;document.querySelector('#projectForm').requestSubmit()`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`uiCalls.filter(call=>call.key==='CreateProject').length`),0);
  assert.ok((await evaluate(`document.querySelector('#projectMessage').textContent`)).includes('10020'));
  await evaluate(`window.uiFailTunnel=false`);
  await evaluate(`window.uiFailBrowserOpen=true`);
  await evaluate(`document.querySelector('#projectForm').requestSubmit()`);
  await new Promise(r=>setTimeout(r,100));
  const created=await evaluate(`uiCalls.filter(call=>call.key==='CreateProject').at(-1)`);
  assert.deepEqual(created.args,['ui-session','/srv/nekssh/src','my-app','go']);
  assert.equal(await evaluate(`document.querySelector('#projectDialog').open`),true);
  assert.ok((await evaluate(`document.querySelector('#projectMessage').textContent`)).includes('无需重复创建'));
  await evaluate(`window.uiFailBrowserOpen=false;document.querySelector('#projectForm').requestSubmit()`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`uiCalls.filter(call=>call.key==='CreateProject').length`),1);
  assert.deepEqual((await evaluate(`uiCalls.filter(call=>call.key==='StartPortTunnel').at(-1)`)).args,['ui-session','tcp','127.0.0.1',10020]);
  assert.equal(await evaluate(`document.querySelector('#projectDialog').open`),false);
  assert.equal(await evaluate(`document.querySelector('#fileList .regular-file')._file.Path`),'/srv/nekssh/src/my-app/main.go');
  await evaluate(`document.querySelector('#fileList .regular-file').dispatchEvent(new MouseEvent('dblclick',{bubbles:true}))`);
  await new Promise(r=>setTimeout(r,100));
  assert.equal(await evaluate(`document.querySelector('#editorDialog').open`),true);
  assert.equal(await evaluate(`document.querySelector('#editorPath').textContent`),'/srv/nekssh/src/my-app/main.go');
  await evaluate(`__ui.editor.dispatch({changes:{from:__ui.editor.state.doc.length,insert:'// edited in NekSSH'}});document.querySelector('#editorSave').click()`);
  await new Promise(r=>setTimeout(r,100));
  const saved=await evaluate(`uiCalls.filter(call=>call.key==='SaveRemoteFile').at(-1)`);
  assert.equal(saved.args[0],'ui-session');assert.equal(saved.args[1],'/srv/nekssh/src/my-app/main.go');assert.ok(saved.args[2].endsWith('// edited in NekSSH'));
  assert.deepEqual(await evaluate(`uiBrowserOpens`),['http://127.0.0.1:54321/?folder=%2Fsrv%2Fnekssh%2Fsrc%2Fmy-app']);
  assert.equal(await evaluate(`Boolean(document.querySelector('#projectOpen'))`),false);
  console.log(JSON.stringify({passed:true,artifacts,checks:['welcome','connection dialog','file selection','file panel dragging','ports','prompt composition','sidebar toggle','scrollback preserved','1024px and 900px layouts','flyout bounds','settings preview and cancel','invalid input focus','saved terminal appearance and timers','settings persist after reload','restore defaults preserves connections','selected directory and context project creation','project enters file list','native SSH editor read/save','VS Code project folder opened, file editing opens no additional browser']}));
}finally{ws?.close();child.kill();await server.close()}
