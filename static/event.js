(function(){
  const select = document.getElementById('comicSelect');
  const info = document.getElementById('comicInfo');
  const feeInput = document.getElementById('feeOverride');
  if(!select || !info) return;
  function update(){
    const opt = select.options[select.selectedIndex];
    const bio = opt.dataset.bio || '';
    const notes = opt.dataset.notes || '';
    const fee = opt.dataset.fee || '';
    info.textContent = bio || notes || '';
    feeInput.placeholder = fee;
  }
  select.addEventListener('change', update);
  update();
})();
