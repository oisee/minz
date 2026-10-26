// Called by build_books.py. Chromium retains vector SVG in the PDF and makes
// raster diagram copies for EPUB readers, which need no Mermaid runtime.
import fs from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
const cfg=JSON.parse(await fs.readFile(process.argv[2],'utf8'));
const pkg=JSON.parse(await fs.readFile(cfg.modules+'/puppeteer/package.json','utf8'));
const {default:puppeteer}=await import(pathToFileURL(cfg.modules+'/puppeteer/'+pkg.main).href);
const browser=await puppeteer.launch({executablePath:cfg.chrome,headless:true,args:['--disable-gpu']});
const page=await browser.newPage();
const results=[];
for(const file of cfg.figures){
 await page.setViewport({width:1600,height:1600,deviceScaleFactor:2});
 await page.goto(pathToFileURL(file.svg).href,{waitUntil:'networkidle0'});
 await (await page.$('svg')).screenshot({path:file.png});
}
for(const ch of cfg.chapters){
 await page.setViewport({width:1440,height:1000,deviceScaleFactor:1});
 await page.goto(pathToFileURL(ch.html).href,{waitUntil:'networkidle0'});
 await page.evaluate(()=>{
  document.querySelectorAll('.sitebar,.progress,.toc,footer,.meta,.next,.playground,figure details').forEach(e=>e.remove());
  document.querySelectorAll('article>p').forEach(e=>{if(e.textContent.trim().startsWith('Все три статьи'))e.remove()});
  document.querySelectorAll('figure.diagram').forEach(fig=>{
   const img=fig.querySelector('img');const xml=new DOMParser().parseFromString(atob(img.src.split(',')[1]),'image/svg+xml');const svg=xml.documentElement;
   const [x,y,w,h]=svg.getAttribute('viewBox').split(/\s+/).map(Number);
   fig.classList.add('book-figure');img.style.width='';
   if(w>780&&h>500){
    // Choose a cut near the centre that does not bisect a text box.
    const probe=document.importNode(svg,true);probe.style.position='absolute';probe.style.opacity='0';probe.style.width=w+'px';probe.style.height=h+'px';document.body.append(probe);
    const inv=probe.getScreenCTM().inverse();const spans=[...probe.querySelectorAll('text')].map(e=>{const b=e.getBBox(),m=inv.multiply(e.getScreenCTM());const p=new DOMPoint(b.x,b.y).matrixTransform(m),q=new DOMPoint(b.x+b.width,b.y+b.height).matrixTransform(m);return [Math.min(p.y,q.y)-4,Math.max(p.y,q.y)+4]});
    let cut=y+h/2;
    for(let d=0;d<h*.16;d++){let found=false;for(const sign of [1,-1]){const c=y+h/2+sign*d;if(!spans.some(([a,b])=>a<c&&c<b)){cut=c;found=true;break}}if(found)break}
    probe.remove();
    const caption=fig.querySelector('figcaption').textContent.split(' · ')[0];
    for(const [i,start,end] of [[1,y,cut],[2,cut,y+h]]){
     const clone=fig.cloneNode(true),copy=svg.cloneNode(true);copy.setAttribute('viewBox',[x,start,w,end-start].join(' '));copy.setAttribute('width',w);copy.setAttribute('height',end-start);copy.style.maxWidth='none';
     clone.querySelector('img').src='data:image/svg+xml;base64,'+btoa(new XMLSerializer().serializeToString(copy));clone.classList.add('book-wide');clone.querySelector('figcaption').textContent=caption+' / часть '+i+' из 2';fig.before(clone);
    }
    fig.remove();
   }else if(w/h>1.35){fig.classList.add('book-wide')}
  });
  document.querySelectorAll('article>p>img').forEach(img=>{const p=img.parentElement,fig=document.createElement('figure');fig.className='book-figure book-wide illustration';p.before(fig);fig.append(img);p.remove()});
  const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);let n;while(n=walker.nextNode()){if(n.parentElement?.tagName!=='SCRIPT')n.nodeValue=n.nodeValue.replace(/[\u2011\u2013\u2014]/g,'-')}
 });
 await page.addStyleTag({content:cfg.css});
 await page.emulateMediaType('print');
 await page.pdf({path:ch.pdf,preferCSSPageSize:true,printBackground:true,displayHeaderFooter:false,tagged:true});
 results.push({chapter:ch.html,pdf:ch.pdf});
}
await browser.close();
console.log(JSON.stringify({rendered:results,figures:cfg.figures.length}));
