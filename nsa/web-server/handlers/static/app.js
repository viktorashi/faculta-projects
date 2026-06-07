// State Management
let satellites = [];
let activeTab = 'catalog';
let animationFrameId = null;
let currentEditingId = null;

// DOM Elements
const tabBtns = document.querySelectorAll('.nav-item');
const tabContents = document.querySelectorAll('.tab-content');
const pageTitle = document.getElementById('page-title');
const pageSubtitle = document.getElementById('page-subtitle');
const toastContainer = document.getElementById('toast-container');

// Stats Elements
const statsTotal = document.getElementById('stats-total');

// Catalog Elements
const catalogSearch = document.getElementById('catalog-search');
const catalogSort = document.getElementById('catalog-sort');
const satellitesContainer = document.getElementById('satellites-container');


// Form Elements
const satelliteForm = document.getElementById('satellite-form');
const formTitleText = document.getElementById('form-title-text');
const formSubtitleText = document.getElementById('form-subtitle-text');
const formSubmitBtn = document.getElementById('form-submit-btn');
const formCancelBtn = document.getElementById('form-cancel-btn');
const inputId = document.getElementById('sat-id');
const inputName = document.getElementById('sat-name');
const inputAxis = document.getElementById('sat-axis');
const inputEcc = document.getElementById('sat-ecc');
const inputInc = document.getElementById('sat-inc');
const inputLan = document.getElementById('sat-lan');
const inputPerigee = document.getElementById('sat-perigee');
// Tab Routing
tabBtns.forEach(btn => {
  btn.addEventListener('click', (e) => {
    e.preventDefault();
    const tab = btn.getAttribute('data-tab');
    switchTab(tab);
  });
});

function switchTab(tabName) {
  activeTab = tabName;

  // Toggle active classes
  tabBtns.forEach(btn => {
    if (btn.getAttribute('data-tab') === tabName) {
      btn.classList.add('active');
    } else {
      btn.classList.remove('active');
    }
  });

  tabContents.forEach(content => {
    if (content.id === `tab-${tabName}`) {
      content.classList.add('active');
    } else {
      content.classList.remove('active');
    }
  });

  // Update Header Text
  if (tabName === 'catalog') {
    pageTitle.innerText = 'Satellite Catalog';
    pageSubtitle.innerText = 'Manage and track orbital paths in the catalog';
    loadSatellites();
  } else if (tabName === 'add') {
    if (currentEditingId) {
      pageTitle.innerText = 'Edit Satellite';
      pageSubtitle.innerText = `Updating record ID: ${currentEditingId}`;
    } else {
      pageTitle.innerText = 'Register Satellite';
      pageSubtitle.innerText = 'Enter orbital metrics to add a spacecraft to the tracking list';
      resetForm();
    }
  }
}

// Toast Notifications
function showToast(message, type = 'info') {
  const toast = document.createElement('div');
  toast.className = `toast ${type}`;

  let iconClass = 'fa-circle-info';
  if (type === 'success') iconClass = 'fa-circle-check';
  if (type === 'error') iconClass = 'fa-triangle-exclamation';
  if (type === 'warning') iconClass = 'fa-circle-exclamation';

  toast.innerHTML = `
        <i class="fa-solid ${iconClass}"></i>
        <span>${message}</span>
    `;

  toastContainer.appendChild(toast);

  // Slide out after 3.5s
  setTimeout(() => {
    toast.style.animation = 'slide-in 0.3s cubic-bezier(0.16, 1, 0.3, 1) reverse forwards';
    setTimeout(() => toast.remove(), 300);
  }, 3500);
}

// Fetch Satellites
async function loadSatellites() {
  try {
    const response = await fetch('/api/satellites');
    if (!response.ok) throw new Error('Failed to retrieve satellites');
    satellites = await response.json();

    // Update stats
    statsTotal.innerText = satellites.length;

    renderCatalog();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// Render Catalog
function renderCatalog() {
  satellitesContainer.innerHTML = '';

  if (satellites.length === 0) {
    satellitesContainer.innerHTML = `
            <div class="empty-state">
                <i class="fa-solid fa-satellite-dish"></i>
                <p>No satellites registered in the database. Add one to get started.</p>
                <button class="btn btn-primary" onclick="switchTab('add')">Register First Satellite</button>
            </div>
        `;
    return;
  }

  // Filter & Sort
  const searchVal = catalogSearch.value.toLowerCase().trim();
  let filtered = satellites.filter(s => {
    return s.name.toLowerCase().includes(searchVal) ||
      s.id.toString() === searchVal ||
      s.semimajor_axis.toString().includes(searchVal) ||
      s.eccentricity.toString().includes(searchVal);
  });

  const sortBy = catalogSort.value;
  filtered.sort((a, b) => {
    if (sortBy === 'id-desc') return b.id - a.id;
    if (sortBy === 'name-asc') return a.name.localeCompare(b.name);
    if (sortBy === 'axis-desc') return b.semimajor_axis - a.semimajor_axis;
    if (sortBy === 'ecc-desc') return b.eccentricity - a.eccentricity;
    return 0;
  });

  if (filtered.length === 0) {
    satellitesContainer.innerHTML = `
            <div class="empty-state">
                <i class="fa-solid fa-magnifying-glass"></i>
                <p>No satellites match your search filters.</p>
            </div>
        `;
    return;
  }

  filtered.forEach(s => {
    const card = document.createElement('div');
    card.className = 'satellite-card';
    card.dataset.id = s.id;

    card.innerHTML = `
            <div class="card-header">
                <h3>${escapeHTML(s.name)}</h3>
                <span class="card-id">#${s.id}</span>
            </div>
            <div class="card-orbit-preview">
                <canvas class="orbit-canvas" 
                        data-axis="${s.semimajor_axis}" 
                        data-ecc="${s.eccentricity}" 
                        data-inc="${s.inclination}"
                        data-lan="${s.longitude_ascending_node}"
                        data-perigee="${s.argument_of_perigee}"></canvas>
            </div>
            <div class="card-specs">
                <div class="spec-item">
                    <span class="spec-label">Semi-major Axis</span>
                    <span class="spec-val">${formatNum(s.semimajor_axis)} km</span>
                </div>
                <div class="spec-item">
                    <span class="spec-label">Eccentricity</span>
                    <span class="spec-val">${formatNum(s.eccentricity, 4)}</span>
                </div>
                <div class="spec-item">
                    <span class="spec-label">Inclination</span>
                    <span class="spec-val">${formatNum(s.inclination)}°</span>
                </div>
                <div class="spec-item">
                    <span class="spec-label">Arg. Perigee</span>
                    <span class="spec-val">${formatNum(s.argument_of_perigee)}°</span>
                </div>
            </div>
            <div class="card-actions">
                <button class="btn-icon edit" onclick="editSatellite(${s.id})" title="Edit Telemetry">
                    <i class="fa-solid fa-pen-to-square"></i>
                </button>
                <button class="btn-icon delete" onclick="deleteSatellite(${s.id})" title="Decommission (Delete)">
                    <i class="fa-solid fa-trash-can"></i>
                </button>
            </div>
        `;

    satellitesContainer.appendChild(card);
  });
}



// Edit Satellite
async function editSatellite(id) {
  try {
    const response = await fetch(`/api/satellites/${id}`);
    if (!response.ok) throw new Error('Could not fetch satellite details');
    const s = await response.json();

    // Fill form
    currentEditingId = id;
    inputId.value = s.id;
    inputName.value = s.name;
    inputAxis.value = s.semimajor_axis;
    inputEcc.value = s.eccentricity;
    inputInc.value = s.inclination;
    inputLan.value = s.longitude_ascending_node;
    inputPerigee.value = s.argument_of_perigee;

    formTitleText.innerText = 'Edit Satellite';
    formSubtitleText.innerText = `Update parameters for satellite #${id}`;
    formSubmitBtn.innerText = 'Save Changes';

    switchTab('add');
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// Delete Satellite
async function deleteSatellite(id) {
  if (!confirm(`Are you sure you want to delete/decommission satellite record #${id}?`)) {
    return;
  }

  try {
    const response = await fetch(`/api/satellites/${id}`, { method: 'DELETE' });
    if (!response.ok) throw new Error('Decommission command rejected by telemetry server');

    showToast(`Satellite #${id} successfully decommissioned and removed.`, 'success');

    // Refresh appropriate view
    if (activeTab === 'catalog') {
      loadSatellites();
    }
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// Form Validation and Submission
satelliteForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  if (!validateForm()) return;

  const payload = {
    name: inputName.value.trim(),
    semimajor_axis: parseFloat(inputAxis.value),
    eccentricity: parseFloat(inputEcc.value),
    inclination: parseFloat(inputInc.value),
    longitude_ascending_node: parseFloat(inputLan.value),
    argument_of_perigee: parseFloat(inputPerigee.value)
  };

  const isEdit = !!currentEditingId;
  const url = isEdit ? `/api/satellites/${currentEditingId}` : '/api/satellites';
  const method = isEdit ? 'PUT' : 'POST';

  try {
    const response = await fetch(url, {
      method: method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    if (response.status === 409) {
      const errData = await response.json();
      return;
    }

    if (!response.ok) {
      const txt = await response.text();
      throw new Error(txt || 'Failed to save satellite record');
    }

    const savedSat = await response.json();
    showToast(`Satellite "${savedSat.name}" saved successfully as ID #${savedSat.id}!`, 'success');

    resetForm();
    switchTab('catalog');
  } catch (err) {
    showToast(err.message, 'error');
  }
});

formCancelBtn.addEventListener('click', () => {
  resetForm();
  switchTab('catalog');
});

function resetForm() {
  currentEditingId = null;
  inputId.value = '';
  satelliteForm.reset();
  formTitleText.innerText = 'Register New Satellite';
  formSubtitleText.innerText = 'Enter the orbital elements to log the spacecraft';
  formSubmitBtn.innerText = 'Register Satellite';

  // Clear errors
  document.querySelectorAll('.error-msg').forEach(el => el.innerText = '');
}

function validateForm() {
  let isValid = true;

  // Clear errors
  document.querySelectorAll('.error-msg').forEach(el => el.innerText = '');

  if (inputName.value.trim() === '') {
    document.getElementById('err-sat-name').innerText = 'Satellite name is required';
    isValid = false;
  }

  const axis = parseFloat(inputAxis.value);
  if (isNaN(axis) || axis <= 6378) {
    document.getElementById('err-sat-axis').innerText = 'Axis must be greater than Earth radius (6,378 km)';
    isValid = false;
  }

  const ecc = parseFloat(inputEcc.value);
  if (isNaN(ecc) || ecc < 0 || ecc >= 1) {
    document.getElementById('err-sat-ecc').innerText = 'Eccentricity must be in range [0.0, 1.0)';
    isValid = false;
  }

  const inc = parseFloat(inputInc.value);
  if (isNaN(inc) || inc < 0 || inc > 180) {
    document.getElementById('err-sat-inc').innerText = 'Inclination must be in range [0, 180] degrees';
    isValid = false;
  }

  const lan = parseFloat(inputLan.value);
  if (isNaN(lan) || lan < 0 || lan > 360) {
    document.getElementById('err-sat-lan').innerText = 'LAN must be in range [0, 360] degrees';
    isValid = false;
  }

  const perigee = parseFloat(inputPerigee.value);
  if (isNaN(perigee) || perigee < 0 || perigee > 360) {
    document.getElementById('err-sat-perigee').innerText = 'Argument of perigee must be in range [0, 360] degrees';
    isValid = false;
  }

  return isValid;
}


// Filter handlers
catalogSearch.addEventListener('input', renderCatalog);
catalogSort.addEventListener('change', renderCatalog);

// Animation Loop for Orbit canvases
function startOrbitAnimations() {
  function animate() {
    // Find all visible canvases on the page
    const canvases = document.querySelectorAll('.orbit-canvas');

    canvases.forEach(canvas => {
      const a = parseFloat(canvas.dataset.axis);
      const e = parseFloat(canvas.dataset.ecc);
      const inc = parseFloat(canvas.dataset.inc);
      const lan = parseFloat(canvas.dataset.lan);
      const perigee = parseFloat(canvas.dataset.perigee);

      // Adjust canvas sizes to avoid scaling distortions
      if (canvas.width !== canvas.clientWidth || canvas.height !== canvas.clientHeight) {
        canvas.width = canvas.clientWidth;
        canvas.height = canvas.clientHeight;
      }

      drawOrbit(canvas, a, e, inc, lan, perigee);
    });

    animationFrameId = requestAnimationFrame(animate);
  }
  animate();
}

function drawOrbit(canvas, a, e, inclination, lan, perigee) {
  const ctx = canvas.getContext('2d');
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const cx = w / 2;
  const cy = h / 2;

  // Draw grid rings
  ctx.beginPath();
  ctx.arc(cx, cy, Math.min(w, h) * 0.35, 0, 2 * Math.PI);
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.02)';
  ctx.stroke();

  // Draw Earth
  const earthGrad = ctx.createRadialGradient(cx, cy, 2, cx, cy, 7);
  earthGrad.addColorStop(0, '#00f2fe');
  earthGrad.addColorStop(1, '#0055ff');
  ctx.beginPath();
  ctx.arc(cx, cy, 6, 0, 2 * Math.PI);
  ctx.fillStyle = earthGrad;
  ctx.shadowColor = 'rgba(0, 242, 254, 0.6)';
  ctx.shadowBlur = 8;
  ctx.fill();
  ctx.shadowBlur = 0;

  // Scale Orbit
  let r_max = Math.min(w, h) * 0.4;
  let scale = r_max / 42164; // GEO scale
  let major = a * scale;
  if (major < 12) major = 12; // LEO visibility
  if (major > r_max) major = r_max; // GEO clamp

  let minor = major * Math.sqrt(1 - e * e);
  let c = major * e; // distance from focus to center

  ctx.save();
  ctx.translate(cx, cy);
  ctx.rotate((perigee + lan) * Math.PI / 180);

  // Draw Orbit Path
  ctx.beginPath();
  ctx.ellipse(-c, 0, major, minor, 0, 0, 2 * Math.PI);
  ctx.strokeStyle = 'rgba(0, 242, 254, 0.25)';
  ctx.lineWidth = 1.25;
  ctx.stroke();

  // Draw Perigee Dot (Gold)
  ctx.beginPath();
  ctx.arc(major - c, 0, 2.5, 0, 2 * Math.PI);
  ctx.fillStyle = '#ffb800';
  ctx.fill();

  // Draw Moving Satellite
  // Calculate orbital period representation
  // T = 2 * pi * sqrt(a^3 / mu). We will simplify to a rate dependent on 'a'.
  let speed = 1.0;
  if (a > 0) speed = Math.sqrt(10000 / a); // LEO moves faster than GEO
  let t = (Date.now() / 1500 * speed) % (2 * Math.PI);

  // Solve Kepler's Eq (simplified for rendering - standard circular/eccentric projection)
  // For visual representation, we can trace by true anomaly on the ellipse
  let satX = major * Math.cos(t) - c;
  let satY = minor * Math.sin(t);

  ctx.beginPath();
  ctx.arc(satX, satY, 3.5, 0, 2 * Math.PI);
  ctx.fillStyle = '#00f5a0';
  ctx.shadowColor = '#00f5a0';
  ctx.shadowBlur = 6;
  ctx.fill();
  ctx.shadowBlur = 0;

  ctx.restore();
}

// Helpers
function escapeHTML(str) {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function formatNum(num, decimals = 1) {
  if (num === undefined || num === null) return '0';
  return Number(num).toLocaleString(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: decimals
  });
}

// Initializer
window.addEventListener('DOMContentLoaded', () => {
  loadSatellites();
  startOrbitAnimations();
});
