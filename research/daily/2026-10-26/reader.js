const progress=document.querySelector('.progress');
function readingProgress(){const d=document.documentElement;const max=d.scrollHeight-d.clientHeight;if(progress)progress.style.width=(max>0?100*d.scrollTop/max:0)+'%'}
addEventListener('scroll',readingProgress,{passive:true});readingProgress();
const demo=document.querySelector('#playground');
if(demo){
 const runs=demo.querySelector('[name=runs]'), demand=demo.querySelector('[name=demand]');
 function simulate(){
  const output=demo.querySelector('.output'), stats=demo.querySelector('.model-stats'), boxes=demo.querySelector('.runs');
  try{
   const pairs=runs.value.split(',').map(s=>{const m=s.trim().match(/^(\d+)\s*:\s*(\d+)$/);if(!m)throw Error('Пары записываются как count:value, например 3:65.');const p=m.slice(1).map(Number);if(p.some(x=>x<0||x>255))throw Error('В этом примере count и value — байты от 0 до 255.');return p});
   if(pairs.length>16)throw Error('Для наглядности — не более 16 пар.');
   const need=Number(demand.value);if(!Number.isInteger(need)||need<0||need>64)throw Error('Спрос — целое число от 0 до 64.');
   const emitted=[];let at=0,left=0,value=0,loaded=0;
   while(emitted.length<need){
    if(left===0){if(at===pairs.length)break;[left,value]=pairs[at++];loaded++;if(left===0)continue;}
    emitted.push(value);left--;
   }
   boxes.replaceChildren();pairs.forEach((p,i)=>{const b=document.createElement('span');b.className='run'+(i<loaded?' read':'');b.textContent=p[0]+' × '+p[1]+(i<loaded?' · прочитано':' · не трогали');boxes.append(b)});
   stats.replaceChildren();[['Прочитано входных байтов',loaded*2],['Выдано',emitted.length],['Осталось в текущем run',left]].forEach(([label,n])=>{const s=document.createElement('span');s.textContent=label+': '+n;stats.append(s)});
   output.className='output';output.textContent=emitted.length?emitted.map(x=>x.toString(16).toUpperCase().padStart(2,'0')).join(' '):'Пока не потребовали ни одного байта.';
  }catch(e){output.className='output error';output.textContent=e.message;stats.replaceChildren();boxes.replaceChildren();}
 }
 demo.querySelector('button').addEventListener('click',simulate);simulate();
}
