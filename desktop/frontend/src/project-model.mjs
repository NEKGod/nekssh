export const projectTemplates=Object.freeze([
  {id:'empty',label:'空项目',files:[],hint:'只创建目录，自由开始。'},
  {id:'go',label:'Go',files:['go.mod','main.go','.gitignore','README.md'],hint:'标准库 HTTP 服务 · go run .'},
  {id:'node',label:'Node.js',files:['package.json','server.mjs','.gitignore','README.md'],hint:'无第三方依赖 · npm start'},
  {id:'python',label:'Python',files:['main.py','.gitignore','README.md'],hint:'标准库 HTTP 服务 · python3 main.py'}
]);

export function validProjectName(name){return /^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(name)}


export function vscodeProjectURL(address,folder,protocol='http'){
  if(!['http','https'].includes(protocol)||!/^\//.test(folder)||folder.includes('\0'))throw Error('无效的 VS Code 项目地址');
  if(!/^(?:\[[\da-fA-F:]+\]|[\w.-]+):\d+$/.test(address))throw Error('无效的 Listen 本地地址');
  const url=new URL(`${protocol}://${address}`);
  if(url.hostname==='0.0.0.0')url.hostname='127.0.0.1';
  if(url.hostname==='[::]')url.hostname='[::1]';
  url.searchParams.set('folder',folder);
  return url.href;
}
