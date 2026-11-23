/* OrgAxis v0.3 presentation workbench. Uses the existing real API and seeded SQLite rows. */
'use strict';
state.oaLayout = 'units';
state.oaFocus = null;
state.oaSelected = null;
state.oaPayrollTab = 'ledger';
state.oaPaySearch = '';
state.oaPayPage = 0;
state.oaBonusFactor = 1;
const unitLabels = {CORP:'Văn phòng công ty', OPS:'Khối vận hành',P1:'Nhà máy 1',P2:'Nhà máy 2',WH:'Phòng Kho',PACK:'Đóng bao',MTN:'Bảo trì',HSE:'An toàn & Môi trường',LOG:'Logistics'};
const money = x => new Intl.NumberFormat('vi-VN',{maximumFractionDigits:0}).format(Number(x||0));
const vnd = x => money(x)+' ₫';
const mon = x => money(Math.round(Number(x||0)/1000000*10)/10)+' tr';
const orgDate = () => state.asof;
const fancyTitle=(title,caption,right='')=>`<div class="oa-page-title"><div class="oa-overline">ORGAXIS · WORKFORCE COMMAND CENTER</div><h1>${title}</h1><p>${caption}</p></div>${right}`;
const tabControl = (items,active,attr)=>`<div class="oa-tabs" role="tablist">${items.map(([id,text])=>`<button type="button" role="tab" ${attr}="${id}" aria-selected="${id===active}" class="${id===active?'selected':''}">${text}</button>`).join('')}</div>`;
const orgNodeTitle=(n,kind)=> kind==='units'?(unitLabels[n.code]||n.name):n.title;
const topUnit=(b,code)=>b.units.find(u=>u.code===code)?.id;
const activeOrgSubtree=(b,layout)=>{
  const units=layout==='units';
  const nodes=units?b.units:b.positions;
  const edges=units?b.org_edges.map(e=>[e.parent_id,e.child_id]):b.positions.filter(p=>p.supervisor_position_id).map(p=>[p.supervisor_position_id,p.id]);
  const child=new Map(nodes.map(n=>[n.id,[]]));const par=new Map();
  for(const [p,c] of edges){if(child.has(p)&&child.has(c)){child.get(p).push(c);par.set(c,p)}}
  const map=new Map(nodes.map(n=>[n.id,n]));
  for (const [k,arr] of child) arr.sort((a,b)=>{
    const na=map.get(a),nb=map.get(b);return (na.code||'').localeCompare(nb.code||'')});
  const roots=nodes.filter(n=>!par.has(n.id));
  return {nodes,child,par,map,roots};
};
function oaUnitSummary(b,staff,id){
 const p=b.positions.filter(p=>p.org_unit_id===id);let total=0,leaders=[];
 for(const pos of p){const people=staff[pos.code]||[];total+=people.length;if(pos.holder_limit===1&&people.length){leaders.push(people[0])}}
 return {positions:p.length,people:total,leaders,limit:p.reduce((n,z)=>n+Number(z.holder_limit||0),0)};
}
function oaOrgGraph(b,staff){
 const kind=state.oaLayout||'units',g=activeOrgSubtree(b,kind);
 if(!g.nodes.length)return '<div class="oa-empty">Không có dữ liệu tổ chức.</div>';
 let focus=g.map.get(state.oaFocus);if(!focus){focus=g.roots[0];state.oaFocus=focus.id}
 const visible=[];const levels=[];const visit=(id,depth)=>{const n=g.map.get(id);if(!n)return;visible.push({id,depth});(levels[depth]??=[]).push(id);if(depth<2){for(const c of g.child.get(id)||[])visit(c,depth+1)}};
 visit(focus.id,0);
 // Centered tidy tree layout based on leaves. Only 3 visible hierarchy levels,
 // deeper branches are intentionally drilled into rather than squeezed onto one SVG.
 const leafIDs=visible.filter(v=>v.depth===2||!(g.child.get(v.id)||[]).some(c=>visible.find(w=>w.id===c))).map(v=>v.id);
 let lead=0;const point=new Map();const yTop=22,dy=158,gap=316;
 function assign(id,d){const children=(g.child.get(id)||[]).filter(c=>visible.some(v=>v.id===c));let x;
   if(children.length){const xs=children.map(c=>assign(c,d+1));x=(xs[0]+xs[xs.length-1])/2}else{x=170+(lead++)*gap}
   point.set(id,{x,y:yTop+d*dy});return x}
 assign(focus.id,0);
 const width=Math.max(760,Math.max(1,lead)*gap+22),height=yTop+3*dy+8;
 const l=[], n=[];
 for (const [id,pt] of point){const p=g.par.get(id);if(!p||!point.has(p))continue;const q=point.get(p);
  l.push(`<path d="M${q.x} ${q.y+122} V${q.y+140} H${pt.x} V${pt.y}" fill="none" stroke="#92a9c2" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/>`);
 }
 for(const [id,pt] of point){const data=g.map.get(id),unit=kind==='units'?data:b.units.find(u=>u.id===data.org_unit_id),isRoot=id===focus.id;
  const stat=kind==='units'?oaUnitSummary(b,staff,id):{positions:1,people:(staff[data.code]||[]).length,limit:Number(data.holder_limit||0),leaders:(staff[data.code]||[]).slice(0,1)};
  const label=orgNodeTitle(data,kind),code=data.code||'';
  const name=state.orgMode==='blank'?'Cơ cấu chưa gắn nhân sự':(stat.leaders[0]?.display_name|| (stat.people>0?`${money(stat.people)} nhân sự đang đảm nhiệm`:'Vị trí khuyết / chưa bố trí'));
  const childCount=(g.child.get(id)||[]).length;
  const fill=isRoot?'#102844':'#ffffff',foreground=isRoot?'#ffffff':'#152f4c',secondary=isRoot?'#c8d9eb':'#576d84';
  const selected=id===state.oaSelected;
  n.push(`<g data-oa-node="${esc(id)}" class="oa-chart-node" role="button" tabindex="0" aria-label="${esc(label)}" transform="translate(${pt.x-137},${pt.y})">
    <rect width="274" height="122" rx="16" fill="${fill}" stroke="${selected?'#E6AC54':isRoot?'#102844':'#cbd8e5'}" stroke-width="${selected?'3':'1.5'}" />
    <rect x="14" y="14" width="40" height="40" rx="12" fill="${isRoot?'#274d70':'#eaf1f7'}"/>
    <text x="34" y="39" font-size="13" font-weight="750" text-anchor="middle" fill="${isRoot?'#ffffff':'#29567e'}">${esc(code.slice(0,3))}</text>
    <text x="64" y="24" font-size="11" letter-spacing=".4" font-weight="750" fill="${secondary}">${kind==='units'?'ĐƠN VỊ':'VỊ TRÍ'} · ${esc(code)}</text>
    <text x="64" y="46" font-size="17" font-weight="800" fill="${foreground}">${esc(short(label,24))}</text>
    <line x1="16" y1="65" x2="258" y2="65" stroke="${isRoot?'#43607c':'#e1e7ee'}"/>
    <text x="16" y="88" font-size="13.5" font-weight="700" fill="${foreground}">${esc(short(name.replace(' (Demo)',''),31))}</text>
    <text x="16" y="107" font-size="12" fill="${secondary}">${kind==='units'?`${money(stat.people)} nhân sự · ${stat.positions} nhóm vị trí`:`${money(stat.people)} / ${money(stat.limit)} định biên`} ${childCount?' · '+childCount+' nhánh':''}</text>
    ${childCount&&visible.find(v=>v.id===id)?.depth===2?`<circle cx="258" cy="8" r="12" fill="#e6ac54"/><text x="258" y="12" text-anchor="middle" font-weight="800" font-size="12" fill="#3c2a0d">+</text>`:''}
   </g>`)
 }
 return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}" width="${Math.round(width*state.zoom)}" height="${Math.round(height*state.zoom)}" class="oa-chart-svg" role="img" aria-label="Sơ đồ tổ chức trực tiếp theo ngày hiệu lực">${l.join('')}${n.join('')}</svg>`;
}
organization=async function(){const [b,o,changes]=await Promise.all([api('/v1/org/blank?as_of='+orgDate()),api('/v1/org/occupied?as_of='+orgDate()+'&limit=1000'),api('/v1/org/changes')]);
 const staff={};for(const a of o.assignments){(staff[a.position_code]??=[]).push(a)}
 state.orgCache={b,o,staff};const g=activeOrgSubtree(b,state.oaLayout),focus=g.map.get(state.oaFocus)||g.roots[0];if(focus)state.oaFocus=focus.id;
 const managers=b.positions.filter(p=>p.holder_limit===1).length;
 const timeline=[['2026-06-30','Trước tái cơ cấu'],['2026-07-01','Chuyển Kho về Khối vận hành'],['2027-01-01','Thêm Logistics']];
 const parent=g.par.get(focus?.id),crumb=parent?`${orgNodeTitle(g.map.get(parent),state.oaLayout)} / `:'';
 const cards=b.units.map(u=>{const s=oaUnitSummary(b,staff,u.id);return `<button class="oa-roster-unit" data-oa-node="${esc(u.id)}" type="button"><div><strong>${esc(unitLabels[u.code]||u.name)}</strong><span>${esc(u.code)} · ${esc(u.unit_type)}</span></div><b>${money(s.people)}</b></button>`}).join('');
 return `<div class="oa-org-workbench">
 ${fancyTitle('Cơ cấu tổ chức theo thời gian','Quản trị cơ cấu trống, nhân sự đang đảm nhiệm và tuyến báo cáo — từ cùng một nguồn dữ liệu.',`<button class="btn soft" data-action="report-change">Đổi quan hệ báo cáo</button>`) }
 <div class="oa-headline-metrics"><div><strong>${money(b.unit_count)}</strong><span>Đơn vị đang hiệu lực</span></div><div><strong>${money(managers)}</strong><span>Vị trí quản lý</span></div><div><strong>${money(o.total)}</strong><span>Nhân sự được phân công</span></div><div><strong>${money(changes.length)}</strong><span>Quyết định thay đổi cơ cấu</span></div></div>
 <div class="oa-content-grid"><div class="oa-chart-workspace">
   <div class="oa-chart-head"><div><h2>Organization Explorer</h2><p>Đang xem tại <strong>${esc(state.asof)}</strong> · ${esc(crumb+orgNodeTitle(focus,state.oaLayout))}</p></div><span class="oa-live-badge">TEMPORAL · LIVE DATA</span></div>
   <div class="oa-chart-controls">${tabControl([['units','Đơn vị'],['positions','Vị trí báo cáo']],state.oaLayout,'data-oa-layout')}${tabControl([['occupied','Có nhân sự'],['blank','Blank org']],state.orgMode,'data-orgmode')}
      <div class="oa-chart-zoom"><button type="button" data-action="org-minus" aria-label="Thu nhỏ">−</button><b>${Math.round(state.zoom*100)}%</b><button type="button" data-action="org-plus" aria-label="Phóng to">+</button><button type="button" data-action="org-fit" aria-label="Đặt lại">Fit</button></div></div>
   <div class="oa-date-rail">${timeline.map(([day,label])=>`<button data-oa-date="${day}" class="${day===state.asof?'is-active':''}"><b>${day.slice(5)}</b><span>${label}</span></button>`).join('')}</div>
   <div class="oa-visual-stage" id="orgView">${oaOrgGraph(b,staff)}</div>
   <div class="oa-graph-footer"><div><span class="oa-dot blue"></span>Quản lý <span class="oa-dot muted"></span>Đơn vị / nhân sự <span class="oa-dot amber"></span>Nhánh có cấp tiếp theo</div><span>Chọn nút trên sơ đồ để xem chi tiết; chọn Tập trung nhánh để đi sâu.</span></div>
   <div id="orgSelection" class="oa-inspector"><div><span class="oa-kicker">CHƯA CHỌN ĐƠN VỊ</span><h3>Khám phá tổ chức bằng cách chọn một nút</h3><p>Thông tin nhân sự và quan hệ cấp trên sẽ xuất hiện tại đây.</p></div></div>
 </div><aside class="oa-org-rail"><div class="oa-rail-head"><span class="oa-kicker">CẤU TRÚC HIỆN TẠI</span><h3>Danh mục đơn vị</h3><p>Chọn để xem chi tiết; cơ cấu thay đổi theo ngày.</p></div><div class="oa-rail-list">${cards}</div><div class="oa-rail-foot"><strong>${money(changes.length)} thay đổi cơ cấu</strong>${changes.map(c=>`<button class="oa-change-entry" data-oa-date="${esc(c.effective_date)}"><b>${esc(c.effective_date)}</b><span>${esc(short(c.title,44))}</span></button>`).join('')}</div></aside></div>
 </div>`;
};
function oaSelectNode(id){const cache=state.orgCache;if(!cache)return;const b=cache.b,staff=cache.staff,kind=state.oaLayout;
 let d=kind==='units'?b.units.find(u=>u.id===id):b.positions.find(p=>p.id===id);if(!d)return;
 state.oaSelected=id;
 const g=activeOrgSubtree(b,kind),sup=g.map.get(g.par.get(id)),children=g.child.get(id)||[];
 const roster=kind==='units'?b.positions.filter(p=>p.org_unit_id===id):[d];
 const head=orgNodeTitle(d,kind),unit=kind==='units'?d:b.units.find(u=>u.id===d.org_unit_id);
 const total=roster.reduce((v,p)=>v+(staff[p.code]||[]).length,0);
 const peek=roster.filter(p=>p.holder_limit===1).flatMap(p=>(staff[p.code]||[]).slice(0,1));
 const html=`<div class="oa-inspector-body"><div><span class="oa-kicker">${kind==='units'?'ĐƠN VỊ TỔ CHỨC':'VỊ TRÍ CÔNG VIỆC'} · ${esc(d.code)}</span><h3>${esc(head)}</h3><p>${esc(sup?orgNodeTitle(sup,kind):'Đỉnh của tuyến báo cáo')} → ${esc(head)}</p></div><div class="oa-inspector-chips"><b>${money(total)} nhân sự</b><span>${roster.length} nhóm vị trí</span>${children.length?`<span>${children.length} đơn vị trực thuộc</span>`:''}</div><div class="oa-inspector-people">${peek.length?peek.slice(0,5).map(p=>`<button data-person="${esc(p.person_id)}">${demoAvatar(p.person_id)} ${esc(p.display_name)} <span>Hồ sơ →</span></button>`).join(''):'<span class="muted">Không có quản lý đang đảm nhiệm trong node này.</span>'}</div><div class="oa-inspector-actions"><button class="btn" data-oa-focus="${esc(id)}">Tập trung nhánh</button><button class="btn soft" data-oa-unit-people="${esc(unit?.code||'')}">Danh sách nhân sự</button><button class="btn soft" data-oa-reset="true">Toàn công ty</button></div></div>`;
 const sel=$('orgSelection');if(sel)sel.innerHTML=html;
 document.querySelectorAll('.oa-chart-node').forEach(x=>x.classList.toggle('is-selected',x.dataset.oaNode===id));
}
function oaRefreshGraph(){let c=state.orgCache;if(c&&$('orgView'))$('orgView').innerHTML=oaOrgGraph(c.b,c.staff)};
const oaPrevExec=executeAction;executeAction=async function(t){let a=t.dataset.action;if(['org-fit','org-plus','org-minus'].includes(a)){
 if(a==='org-fit')state.zoom=.87;
 if(a==='org-plus')state.zoom=Math.min(1.5,Math.round((state.zoom+.13)*100)/100);
 if(a==='org-minus')state.zoom=Math.max(.55,Math.round((state.zoom-.13)*100)/100);
 oaRefreshGraph();return;
}return oaPrevExec(t)};
state.zoom=.87;
// ---------------- Payroll console ----------------
payroll=async function(){const [x,benefits]=await Promise.all([api('/v1/payroll/workbench?period='+state.period+'&limit=40&offset='+(state.oaPayPage*40)+'&q='+encodeURIComponent(state.oaPaySearch)),api('/v1/benefits/claims')]);
 const sum=x.summary||{};
 const status={'2026-09':'Đối chiếu xong (giả lập)','2026-10':'Đang kiểm tra (giả lập)','2026-11':'Nháp (giả lập)'}[x.period]||'Chưa có dữ liệu';
 const steps=['Thu thập','Tổng hợp','Kiểm tra','Phê duyệt','Chốt sổ'];
 let main='';
 if(state.oaPayrollTab==='ledger'){
 const rows=x.entries.map((p,i)=>`<tr><td><div class="oa-pay-person"><span>${esc((p.display_name||'P').split(' ').slice(-1)[0].slice(0,2))}</span><div><strong>${esc(p.display_name)}</strong><small>${esc(p.person_id)}</small></div></div></td><td class="number">${mon(p.base_pay)}</td><td class="number">${mon(p.allowance+p.overtime)}</td><td class="number oa-highlight">${mon(p.performance_bonus+p.attendance_bonus)}</td><td class="number">${mon(p.gross)}</td><td class="number">${mon(p.employee_insurance+p.pit_estimate+p.other_deduction)}</td><td class="number oa-pay-net">${mon(p.net_estimate)}</td><td><button class="btn soft small" data-oa-payslip="${esc(p.person_id)}">Chi tiết</button></td></tr>`);
 main=`<section class="oa-section"><div class="oa-section-head"><div><h2>Sổ lương theo nhân viên</h2><p>${money(x.total)} bản ghi phù hợp · Mỗi dòng có thể mở bảng lương chi tiết.</p></div><div class="oa-filter-row"><input class="input" id="oaPaySearch" value="${esc(state.oaPaySearch)}" placeholder="Tên / mã nhân viên"><button class="btn" data-oa-pay-filter="1">Tìm</button></div></div>${tbl(['NHÂN SỰ','CƠ BẢN','PHỤ CẤP + OT','THƯỞNG','TỔNG THU','KHẤU TRỪ','THỰC NHẬN*',''],rows)}<div class="oa-pager"><button class="btn soft" data-oa-paypage="${state.oaPayPage-1}" ${state.oaPayPage===0?'disabled':''}>← Trước</button><span>Trang ${state.oaPayPage+1} / ${Math.max(1,Math.ceil(x.total/40))}</span><button class="btn soft" data-oa-paypage="${state.oaPayPage+1}" ${(state.oaPayPage+1)*40>=x.total?'disabled':''}>Sau →</button></div><p class="oa-footnote">* “Thực nhận” là kết quả mô phỏng. Không phải bảng lương thực hoặc số thuế phải nộp.</p></section>`;
 }else if(state.oaPayrollTab==='bonus'){
 const list=x.entries.slice().sort((a,b)=>(b.performance_bonus+b.attendance_bonus)-(a.performance_bonus+a.attendance_bonus));
 const units=x.units||[], trend=x.trend||[], maxUnit=Math.max(1,...units.map(u=>Number(u.bonus)));
 const baseline=Number(sum.bonuses||0),scenario=Math.round(baseline*state.oaBonusFactor);
 main=`<div class="oa-pay-columns"><section class="oa-section"><h2>Ngân sách thưởng theo đơn vị</h2><p>Được tổng hợp từ phân công có hiệu lực trong kỳ, không phải giả lập bằng hình minh họa.</p>
 ${units.map(u=>`<div class="oa-budget-row"><div><strong>${esc(unitLabels[u.unit_code]||u.unit_name)}</strong><span>${money(u.staff)} người · Bình quân lương ${mon(u.average_gross)}</span></div><div class="oa-budget-bar"><i style="width:${Math.max(4,Number(u.bonus)/maxUnit*100).toFixed(1)}%"></i></div><b>${vnd(u.bonus)}</b></div>`).join('')}
 <div class="oa-reward-note">Các khoản thưởng demo gồm thưởng chuyên cần và hiệu suất. Chỉ số không thay thế quyết định khen thưởng hoặc thưởng theo quy chế.</div></section>
 <section class="oa-section"><h2>Budget what-if</h2><p>Thử phân bổ quỹ thưởng tăng/giảm; kết quả chỉ tính trong trình duyệt, không ghi sổ.</p><div class="oa-bonus-scenario"><span>Quỹ thưởng giả lập</span><strong>${vnd(scenario)}</strong><small>Chênh so với kịch bản gốc: ${vnd(scenario-baseline)}</small></div><div class="oa-bonus-choices">${[[.85,'−15%'],[1,'Baseline'],[1.1,'+10%'],[1.2,'+20%']].map(([f,t])=>`<button class="${Math.abs(state.oaBonusFactor-f)<.001?'selected':''}" data-oa-bonus-factor="${f}">${t}</button>`).join('')}</div><h3>Xu hướng tổng thu nhập</h3><div class="oa-trend-bars">${trend.map(t=>`<div><span>${esc(t.period)}</span><div class="oa-budget-bar"><i style="width:${(Number(t.gross)/Math.max(1,...trend.map(v=>Number(v.gross)))*100).toFixed(1)}%"></i></div><b>${mon(t.gross)} ₫</b></div>`).join('')}</div></section></div>
 <section class="oa-section"><div class="oa-section-head"><div><h2>Chi tiết thưởng nhân viên</h2><p>20 người đứng đầu trong trang dữ liệu đang xem · Mở phiếu lương để đối chiếu cấu phần.</p></div></div>${tbl(['NHÂN SỰ','CHUYÊN CẦN','HIỆU SUẤT','TỔNG THƯỞNG','CHI TIẾT'],list.slice(0,20).map(p=>`<tr><td>${personLink(p.person_id,p.display_name)}</td><td class="number">${vnd(p.attendance_bonus)}</td><td class="number">${vnd(p.performance_bonus)}</td><td class="number"><b>${vnd(p.attendance_bonus+p.performance_bonus)}</b></td><td><button class="btn soft small" data-oa-payslip="${esc(p.person_id)}">Xem phiếu lương</button></td></tr>`))}</section>`;
 }else if(state.oaPayrollTab==='tax'){
 main=`<div class="oa-pay-columns"><section class="oa-section"><h2>Mô phỏng thuế & các khoản trừ</h2><p>Điều chỉnh dữ liệu đầu vào, xem cách phân tách các khoản. Công thức minh họa, không phải quy định thuế Việt Nam hiện hành.</p><div class="oa-sim-form"><label>Tổng thu nhập tháng (VND)<input type="number" id="oaGross" value="30000000" min="0" max="1000000000"></label><label>Lương làm căn cứ giả định (VND)<input type="number" id="oaInsured" value="19000000" min="0"></label><label>Người phụ thuộc<input type="number" id="oaDependents" value="1" min="0" max="25"></label><button class="btn" data-oa-tax-sim="1">Tính thử theo giả định</button></div><div id="oaTaxResult" class="oa-tax-result"><span>Nhập dữ liệu và nhấn Tính thử</span></div></section><section class="oa-section"><h2>Quy tắc dùng trong mô phỏng</h2><div class="oa-rule-list"><div><span>BH người lao động</span><strong>10,5% trên lương giả định</strong></div><div><span>Giảm trừ cá nhân</span><strong>11.000.000 ₫</strong></div><div><span>Giảm trừ người phụ thuộc</span><strong>4.400.000 ₫ / người</strong></div><div><span>Thuế lũy tiến</span><strong>7 bậc mô phỏng</strong></div><div><span>Phiên bản</span><strong>ILLUSTRATIVE-2026</strong></div></div><div class="oa-review-alert"><b>Không phải bộ quy tắc pháp lý</b><p>Đây là sandbox tính thử các dòng thu nhập và khấu trừ. Khi triển khai thật phải cập nhật các khoản giảm trừ, biểu thuế, mức đóng và luật theo ngày có hiệu lực.</p></div></section></div>`;
 }else{
 main=`<div class="oa-pay-columns"><section class="oa-section"><h2>Hồ sơ chế độ</h2><p>Nghỉ ốm, thai sản, tai nạn lao động — quản lý hồ sơ và tình trạng xét duyệt.</p>${tbl(['NHÂN SỰ','CHẾ ĐỘ','NGÀY NỘP','TRẠNG THÁI'],benefits.items.slice(0,25).map(v=>`<tr><td>${personLink(v.person_id,v.display_name)}</td><td>${esc(v.benefit_type)}</td><td>${esc(v.application_date)}</td><td>${badge(v.status)}</td></tr>`))}</section><section class="oa-section"><h2>Quy trình quyền lợi</h2>${['Tiếp nhận hồ sơ','Xác minh nguồn và thời gian BHXH','Đối chiếu điều kiện áp dụng','Phê duyệt / trả lại','Lưu quyết định, chứng từ'].map((v,i)=>`<div class="oa-process-step"><b>${String(i+1).padStart(2,'0')}</b><span>${v}</span></div>`).join('')}</section></div>`;
 }
 return `<div class="oa-payroll-workbench">${fancyTitle('Lương, thưởng & thuế','Một sổ lương có thể đối chiếu theo người, thành phần thu nhập, kỳ lương và các giả định tính toán.')}
 <div class="oa-demo-banner"><span>SYNTHETIC FINANCE DATA</span> Tất cả số tiền là giả lập. Chưa có tính lương, BHXH hoặc PIT theo bộ quy tắc pháp luật đã được xác minh.</div>
 <div class="oa-period-head"><div><span class="oa-kicker">KỲ XỬ LÝ</span><h2>${esc(x.period)} <span>${esc(status)}</span></h2></div><div><button class="btn soft" data-oa-period="2026-09">09/2026</button><button class="btn soft" data-oa-period="2026-10">10/2026</button><button class="btn soft" data-oa-period="2026-11">11/2026</button></div></div>
 <div class="oa-pay-kpis"><div><span>Tổng thu nhập</span><strong>${vnd(sum.gross)}</strong><small>${money(sum.staff)} nhân viên</small></div><div><span>Quỹ thưởng</span><strong>${vnd(sum.bonuses)}</strong><small>Chuyên cần + hiệu suất</small></div><div><span>Thuế mô phỏng</span><strong>${vnd(sum.pit)}</strong><small>Không phải PIT pháp lý</small></div><div><span>Thực nhận mô phỏng</span><strong>${vnd(sum.net)}</strong><small>Sau khấu trừ giả định</small></div></div>
 <div class="oa-pay-pipeline">${steps.map((label,i)=>`<div class="${i===2?'current':''}"><b>${String(i+1).padStart(2,'0')}</b><span>${label}</span></div>`).join('')}</div>
 ${tabControl([['ledger','Sổ lương'],['bonus','Thưởng'],['tax','PIT Simulator'],['benefits','Chế độ']],state.oaPayrollTab,'data-oa-paytab')}
 ${main}</div>`;
};
function oaPayslipHTML(p){const lines=[['Lương cơ bản',p.base_pay,'plus'],['Phụ cấp',p.allowance,'plus'],['Làm thêm giờ',p.overtime,'plus'],['Thưởng chuyên cần',p.attendance_bonus,'plus'],['Thưởng hiệu suất',p.performance_bonus,'plus'],['BH người lao động (mô phỏng)',-p.employee_insurance,'minus'],['Thuế PIT (mô phỏng)',-p.pit_estimate,'minus'],['Khấu trừ khác',-p.other_deduction,'minus']];
 return `<div class="oa-slip"><div class="oa-slip-hero"><div><span class="oa-kicker">DEMO PAYSLIP · ${esc(p.period)}</span><h2>${esc(p.display_name)}</h2><p>${esc(p.person_id)} · Quốc tịch ${esc(p.nationality)}</p></div><span class="oa-slip-icon">₫</span></div><div class="oa-slip-summary"><span>THỰC NHẬN MÔ PHỎNG</span><strong>${vnd(p.net_estimate)}</strong><small>Tổng thu ${vnd(p.gross)} · Khấu trừ ${vnd(p.employee_insurance+p.other_deduction+p.pit_estimate)}</small></div><h3>Các thành phần trong kỳ</h3>${lines.map(([name,amt,k])=>`<div class="oa-slip-line"><span>${esc(name)}</span><strong class="${k==='minus'?'minus':''}">${amt<0?'−':''}${vnd(Math.abs(amt))}</strong></div>`).join('')}<div class="oa-slip-line total"><span>THỰC NHẬN MÔ PHỎNG</span><strong>${vnd(p.net_estimate)}</strong></div><div class="oa-slip-footer"><b>Không phải phiếu lương chính thức.</b><p>Quy tắc: ${esc(p.rule_version)}. Dòng thuế và BH được tính theo giả định để minh họa trải nghiệm giải trình số liệu.</p></div></div>`;
}
immigration=async function(){const r=await api('/v1/immigration/summary?as_of='+state.asof);const foreign=r.experts||[];
 const summary=r.nationalities.filter(n=>n.nationality!=='VN');const countries={JP:'Japan',CN:'China',TW:'Taiwan',KR:'South Korea',IN:'India',US:'United States'};
 return `<div class="oa-immigration">${fancyTitle('Global mobility & chuyên gia quốc tế','Hồ sơ công tác xuyên biên giới: giấy phép, cư trú, doanh nghiệp bảo lãnh và cảnh báo thời hạn.')}
 <div class="oa-demo-banner"><span>ALL SYNTHETIC</span> Danh tính, quốc tịch, số giấy tờ và ngày hết hạn chỉ phục vụ demo; chưa xác minh quyền làm việc hoặc cư trú.</div>
 <div class="oa-headline-metrics"><div><strong>${foreign.length}</strong><span>Chuyên gia quốc tế</span></div><div><strong>${summary.length}</strong><span>Quốc gia / vùng lãnh thổ</span></div><div><strong>${r.expiring_90d}</strong><span>Hồ sơ sắp đến hạn 90 ngày</span></div><div><strong>${foreign.filter(p=>p.authorization_type).length}</strong><span>Giấy phép / miễn cấp</span></div></div>
 <div class="oa-foreign-grid"><div class="oa-section"><div class="oa-section-head"><div><h2>Expert register</h2><p>Nhân sự dùng tên theo quốc tịch, hồ sơ phân tách từng loại giấy tờ.</p></div><span class="oa-live-badge">18 PROFILES</span></div><div class="oa-expert-grid">${foreign.map(p=>`<button class="oa-expert-card" data-oa-expert="${esc(p.id)}"><div class="oa-expert-avatar">${demoAvatar(p.id)}</div><div class="oa-expert-name"><b>${esc(p.display_name.replace(' (Demo)',''))}</b><span>${esc(countries[p.nationality]||p.nationality)} · ${esc(p.id)}</span></div><span class="oa-expert-iso">${esc(p.nationality)}</span></button>`).join('')}</div></div><aside class="oa-section oa-expert-detail" id="oaExpertDetail"><div class="oa-kicker">EXPERT DOSSIER</div><h2>Chọn một chuyên gia</h2><p>Giấy phép lao động, thẻ cư trú, hộ chiếu và phạm vi công việc sẽ hiển thị ở đây.</p><div class="oa-workflow-strip"><div>01 · Xác minh danh tính</div><div>02 · Giấy phép hoặc miễn cấp</div><div>03 · Tình trạng cư trú</div><div>04 · Phạm vi công việc</div></div></aside></div></div>`;
};
function oaExpertSelect(id){let r=state.oaExperts||[];let p=r.find(x=>x.id===id);if(!p)return;const box=$('oaExpertDetail');if(!box)return;
 box.innerHTML=`<span class="oa-kicker">EXPERT DOSSIER · ${esc(p.id)}</span><h2>${esc(p.display_name.replace(' (Demo)',''))}</h2><p>${esc(p.nationality)} · ${esc(p.role_scope||'Engineering consultant')}</p><div class="oa-expert-facts"><div><span>Work authorization</span><b>${esc(p.authorization_type||'Chưa ghi')}</b></div><div><span>Hết hạn giấy phép</span><b>${esc(p.valid_to||'Chưa ghi')}</b></div><div><span>Trạng thái</span><b>${esc(p.permit_status||'unverified_demo')}</b></div><div><span>Worksite</span><b>${esc(p.worksite_scope||'Chưa ghi')}</b></div><div><span>Hộ chiếu (đã che)</span><b>${esc(p.masked_reference||'—')}</b></div><div><span>Hạn cư trú</span><b>${esc(p.residence_valid_to||'—')}</b></div></div><div class="oa-review-alert"><b>Verification required</b><p>Hồ sơ giả lập. Không dùng để kết luận cá nhân được nhập cảnh, cư trú hoặc làm việc hợp pháp.</p></div><button class="btn" data-person="${esc(p.id)}">Xem hồ sơ nhân sự</button>`;
 document.querySelectorAll('.oa-expert-card').forEach(x=>x.classList.toggle('active',x.dataset.oaExpert===id));
}
const oaImm=immigration;immigration=async function(){const r=await api('/v1/immigration/summary?as_of='+state.asof);state.oaExperts=r.experts;return oaImm()};
document.addEventListener('click',async e=>{const t=e.target.closest('[data-oa-node],[data-oa-focus],[data-oa-reset],[data-oa-unit-people],[data-oa-layout],[data-oa-date],[data-oa-paytab],[data-oa-payslip],[data-oa-paypage],[data-oa-pay-filter],[data-oa-period],[data-oa-tax-sim],[data-oa-expert],[data-oa-bonus-factor]');if(!t)return;
 try{
 if(t.dataset.oaNode){oaSelectNode(t.dataset.oaNode);return}
 if(t.dataset.oaFocus){state.oaFocus=t.dataset.oaFocus;state.oaSelected=null;await render();return}
 if(t.dataset.oaReset){state.oaFocus=null;state.oaSelected=null;await render();return}
 if(t.dataset.oaUnitPeople){state.peopleUnit=t.dataset.oaUnitPeople;state.peoplePage=0;await navigate('people');return}
 if(t.dataset.oaLayout){state.oaLayout=t.dataset.oaLayout;state.oaFocus=null;state.oaSelected=null;await render();return}
 if(t.dataset.oaDate){state.asof=t.dataset.oaDate;$('asof').value=state.asof;state.oaFocus=null;state.oaSelected=null;await render();return}
 if(t.dataset.oaBonusFactor){state.oaBonusFactor=Number(t.dataset.oaBonusFactor);await render();return}
 if(t.dataset.oaPaytab){state.oaPayrollTab=t.dataset.oaPaytab;await render();return}
 if(t.dataset.oaPaypage!==undefined){state.oaPayPage=Number(t.dataset.oaPaypage);await render();return}
 if(t.dataset.oaPayFilter){state.oaPayPage=0;state.oaPaySearch=$('oaPaySearch')?.value||'';await render();return}
 if(t.dataset.oaPeriod){state.period=t.dataset.oaPeriod;state.oaPayPage=0;await render();return}
 if(t.dataset.oaPayslip){const d=await api('/v1/payroll/payslip/'+encodeURIComponent(t.dataset.oaPayslip)+'?period='+state.period);$('drawerBackdrop').hidden=false;$('drawer').hidden=false;$('drawerContent').innerHTML='<div class="drawer-content"><button class="close" data-action="close-drawer" aria-label="Close">×</button>'+oaPayslipHTML(d.person)+'</div>';return}
 if(t.dataset.oaTaxSim){let gross=Number($('oaGross').value),insured=Number($('oaInsured').value),deps=Number($('oaDependents').value);let x=await api(`/v1/payroll/simulate?gross=${gross}&insurable_base=${insured}&dependents=${deps}`);$('oaTaxResult').innerHTML=`<div><span>Thu nhập sau giảm trừ (giả lập)</span><b>${vnd(x.illustrative_taxable)}</b></div><div><span>BH người lao động (giả lập)</span><b>${vnd(x.illustrative_insurance)}</b></div><div><span>PIT mô phỏng</span><b>${vnd(x.illustrative_pit)}</b></div><div class="total"><span>THỰC NHẬN MÔ PHỎNG</span><b>${vnd(x.illustrative_net)}</b></div>`;return}
 if(t.dataset.oaExpert){oaExpertSelect(t.dataset.oaExpert);return}
 }catch(err){toast(err.message,true)}
});
document.addEventListener('keydown',e=>{if((e.key==='Enter'||e.key===' ')&&e.target.closest('.oa-chart-node')){e.preventDefault();oaSelectNode(e.target.closest('.oa-chart-node').dataset.oaNode)}if(e.key==='Enter'&&e.target.id==='oaPaySearch'){$('[data-oa-pay-filter]')?.click()}});
