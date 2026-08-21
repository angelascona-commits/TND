const API = 'https://5lbbwct8lj.execute-api.us-east-1.amazonaws.com/prod';

let catalogCache = { CATEGORIAS: [], MARCAS: [], ZONAS: [] };
let catalogsLoaded = false;

function showToast(msg, isError) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast' + (isError ? ' error' : '');
  setTimeout(() => t.className = 'toast hidden', 3500);
}

function closeModal(id) {
  document.getElementById(id).classList.add('hidden');
}

function openModal(id) {
  document.getElementById(id).classList.remove('hidden');
}

function emptyRow(cols, msg) {
  return `<tr class="empty-row"><td colspan="${cols}">${msg || 'Use los filtros para realizar una busqueda.'}</td></tr>`;
}

document.querySelectorAll('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(s => s.classList.add('hidden'));
    btn.classList.add('active');
    document.getElementById(btn.dataset.tab).classList.remove('hidden');

    if (!catalogsLoaded) loadCatalogs();
  });
});

async function loadCatalogs() {
  if (catalogsLoaded) return;
  try {
    const res = await fetch(`${API}/catalogs?categories=CATEGORIAS,MARCAS,ZONAS`);
    if (!res.ok) throw new Error('Error al cargar catalogos');
    const items = await res.json();

    catalogCache = { CATEGORIAS: [], MARCAS: [], ZONAS: [] };
    items.forEach(item => {
      const cat = (item.category || '').toUpperCase();
      if (catalogCache[cat]) {
        catalogCache[cat].push(item);
      }
    });

    populateSelect('ent-zona-select', catalogCache.ZONAS, '-- Todas --');
    populateSelect('fe-zona', catalogCache.ZONAS, '-- Sin zona --');

    populateSelect('prod-cat-select', catalogCache.CATEGORIAS, '-- Todas --');
    populateSelect('fp-categoryId', catalogCache.CATEGORIAS, '-- Seleccione --');

    populateSelect('prod-brand-select', catalogCache.MARCAS, '-- Todas --');
    populateSelect('fp-brandId', catalogCache.MARCAS, '-- Seleccione --');

    catalogsLoaded = true;
  } catch (e) {
    showToast('No se pudieron cargar los catalogos: ' + e.message, true);
  }
}

function populateSelect(selectId, items, defaultText) {
  const sel = document.getElementById(selectId);
  if (!sel) return;
  sel.innerHTML = `<option value="">${defaultText}</option>`;
  items.forEach(item => {
    const val = item.itemId || item.label;
    sel.innerHTML += `<option value="${val}">${item.label || val}</option>`;
  });
}

document.getElementById('btn-search-catalogs').addEventListener('click', searchCatalogs);
document.getElementById('btn-show-create-catalog').addEventListener('click', () => {
  document.getElementById('fc-mode').value = 'create';
  document.getElementById('modal-catalog-title').textContent = 'Nuevo Catalogo';
  document.getElementById('form-catalog').reset();
  document.getElementById('fc-isActive').checked = true;
  openModal('modal-catalog');
});
document.getElementById('form-catalog').addEventListener('submit', saveCatalog);

async function searchCatalogs() {
  const type = document.getElementById('cat-type-select').value;
  if (!type) return showToast('Seleccione un tipo de catalogo');
  const tbody = document.getElementById('catalogs-tbody');
  tbody.innerHTML = emptyRow(6, 'Buscando...');

  try {
    const res = await fetch(`${API}/catalogs?categories=${type}`);
    if (!res.ok) throw new Error('Error al consultar catalogos');
    const data = await res.json();
    if (!data.length) { tbody.innerHTML = emptyRow(6, 'No se encontraron resultados.'); return; }

    tbody.innerHTML = '';
    data.forEach(item => {
      const active = item.isActive ? '<span class="badge badge-green">Si</span>' : '<span class="badge badge-red">No</span>';
      tbody.innerHTML += `<tr>
        <td><span class="badge badge-blue">${item.category || ''}</span></td>
        <td>${item.itemId || ''}</td>
        <td>${item.label || ''}</td>
        <td>${item.sortOrder ?? ''}</td>
        <td>${active}</td>
        <td>
          <button class="btn btn-sm" onclick="editCatalog('${item.category}','${item.itemId}','${(item.label||'').replace(/'/g,"\\'")}',${item.sortOrder||0},${item.isActive})">Editar</button>
          <button class="btn btn-danger btn-sm" onclick="deleteCatalog('${item.category}','${item.itemId}')">Eliminar</button>
        </td>
      </tr>`;
    });
  } catch (e) {
    tbody.innerHTML = emptyRow(6, e.message);
    showToast(e.message, true);
  }
}

function editCatalog(category, itemId, label, sortOrder, isActive) {
  document.getElementById('fc-mode').value = 'edit';
  document.getElementById('modal-catalog-title').textContent = 'Editar Catalogo';
  document.getElementById('fc-category').value = category;
  document.getElementById('fc-itemId').value = itemId;
  document.getElementById('fc-label').value = label;
  document.getElementById('fc-sortOrder').value = sortOrder;
  document.getElementById('fc-isActive').checked = isActive;
  openModal('modal-catalog');
}

async function saveCatalog(e) {
  e.preventDefault();
  const mode = document.getElementById('fc-mode').value;
  const body = {
    category: document.getElementById('fc-category').value,
    itemId: document.getElementById('fc-itemId').value,
    label: document.getElementById('fc-label').value,
    sortOrder: parseInt(document.getElementById('fc-sortOrder').value) || 1,
    isActive: document.getElementById('fc-isActive').checked
  };
  try {
    const method = mode === 'edit' ? 'PATCH' : 'POST';
    const res = await fetch(`${API}/catalogs`, {
      method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error'); }
    showToast(mode === 'edit' ? 'Catalogo actualizado' : 'Catalogo creado');
    closeModal('modal-catalog');
    catalogsLoaded = false;
    await loadCatalogs();
    searchCatalogs();
  } catch (e) { showToast(e.message, true); }
}

async function deleteCatalog(category, itemId) {
  if (!confirm('¿Eliminar este registro del catalogo?')) return;
  try {
    const res = await fetch(`${API}/catalogs`, {
      method: 'DELETE', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ category, itemId })
    });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error'); }
    showToast('Registro eliminado del catalogo');
    searchCatalogs();
  } catch (e) { showToast(e.message, true); }
}

document.getElementById('btn-search-entities').addEventListener('click', searchEntities);
document.getElementById('btn-search-entity-doc').addEventListener('click', searchEntityByDoc);
document.getElementById('btn-show-create-entity').addEventListener('click', () => {
  document.getElementById('fe-mode').value = 'create';
  document.getElementById('modal-entity-title').textContent = 'Nuevo Cliente / Proveedor';
  document.getElementById('form-entity').reset();
  document.getElementById('fe-dniRuc').removeAttribute('readonly');
  openModal('modal-entity');
});
document.getElementById('form-entity').addEventListener('submit', saveEntity);

async function searchEntities() {
  const rol = document.getElementById('ent-rol-select').value;
  const zona = document.getElementById('ent-zona-select').value;
  if (!rol && !zona) return showToast('Seleccione al menos un Tipo o una Zona');

  const tbody = document.getElementById('entities-tbody');
  tbody.innerHTML = emptyRow(7, 'Buscando...');

  let url = `${API}/entities?`;
  if (rol) url += `rol=${rol}&`;
  if (zona) url += `zona=${zona}&`;

  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error('Error al consultar');
    const data = await res.json();
    renderEntities(data);
  } catch (e) {
    tbody.innerHTML = emptyRow(7, e.message);
    showToast(e.message, true);
  }
}

async function searchEntityByDoc() {
  const doc = document.getElementById('ent-doc-input').value.trim();
  if (!doc) return showToast('Ingrese un numero de documento');

  const tbody = document.getElementById('entities-tbody');
  tbody.innerHTML = emptyRow(7, 'Buscando...');

  try {
    const res = await fetch(`${API}/entities/${doc}`);
    if (!res.ok) {
      if (res.status === 404) { tbody.innerHTML = emptyRow(7, 'No se encontro una entidad con ese documento.'); return; }
      throw new Error('Error al consultar');
    }
    const entity = await res.json();
    renderEntities([entity]);
  } catch (e) {
    tbody.innerHTML = emptyRow(7, e.message);
    showToast(e.message, true);
  }
}

function renderEntities(data) {
  const tbody = document.getElementById('entities-tbody');
  if (!data.length) { tbody.innerHTML = emptyRow(7, 'No se encontraron resultados.'); return; }
  tbody.innerHTML = '';
  data.forEach(ent => {
    const rolBadge = ent.rol === 'Proveedor' ? 'badge-blue' : 'badge-green';
    tbody.innerHTML += `<tr>
      <td><strong>${ent.dniRuc || ''}</strong></td>
      <td>${ent.nombre || ''}</td>
      <td><span class="badge ${rolBadge}">${ent.rol || ''}</span></td>
      <td>${ent.zona || '-'}</td>
      <td>${ent.correo || '-'}</td>
      <td>${ent.telefono || '-'}</td>
      <td>
        <button class="btn btn-secondary btn-sm" onclick="viewUserHistory('${ent.dniRuc}')">Historial</button>
        <button class="btn btn-sm" onclick="editEntity('${ent.dniRuc}')">Editar</button>
        <button class="btn btn-danger btn-sm" onclick="deleteEntity('${ent.dniRuc}')">Eliminar</button>
      </td>
    </tr>`;
  });
}

function viewUserHistory(dniRuc) {
  document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
  document.querySelectorAll('.tab-content').forEach(s => s.classList.add('hidden'));
  const transTabBtn = document.querySelector('[data-tab="tab-orders"]');
  if (transTabBtn) transTabBtn.classList.add('active');
  document.getElementById('tab-orders').classList.remove('hidden');

  document.getElementById('ord-user-input').value = dniRuc;
  document.getElementById('ord-filter-type').value = '';
  document.getElementById('ord-from-date').value = '';
  document.getElementById('ord-to-date').value = '';
  searchOrders();
}

async function editEntity(dniRuc) {
  try {
    const res = await fetch(`${API}/entities/${dniRuc}`);
    if (!res.ok) throw new Error('No se pudo obtener la entidad');
    const ent = await res.json();

    document.getElementById('fe-mode').value = 'edit';
    document.getElementById('modal-entity-title').textContent = 'Editar: ' + ent.nombre;
    document.getElementById('fe-dniRuc').value = ent.dniRuc;
    document.getElementById('fe-dniRuc').setAttribute('readonly', true);
    document.getElementById('fe-nombre').value = ent.nombre || '';
    document.getElementById('fe-rol').value = ent.rol || 'Cliente';
    document.getElementById('fe-zona').value = (ent.zona || '').toUpperCase();
    document.getElementById('fe-correo').value = ent.correo || '';
    document.getElementById('fe-telefono').value = ent.telefono || '';
    document.getElementById('fe-direccion').value = ent.direccion || '';
    openModal('modal-entity');
  } catch (e) { showToast(e.message, true); }
}

async function saveEntity(e) {
  e.preventDefault();
  const mode = document.getElementById('fe-mode').value;
  const body = {
    dniRuc: document.getElementById('fe-dniRuc').value.trim(),
    nombre: document.getElementById('fe-nombre').value.trim(),
    rol: document.getElementById('fe-rol').value,
    zona: document.getElementById('fe-zona').value,
    correo: document.getElementById('fe-correo').value.trim(),
    telefono: document.getElementById('fe-telefono').value.trim(),
    direccion: document.getElementById('fe-direccion').value.trim()
  };
  try {
    const method = mode === 'edit' ? 'PATCH' : 'POST';
    const res = await fetch(`${API}/entities`, {
      method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error'); }
    showToast(mode === 'edit' ? 'Entidad actualizada' : 'Entidad creada');
    closeModal('modal-entity');
  } catch (e) { showToast(e.message, true); }
}

async function deleteEntity(dniRuc) {
  if (!confirm('¿Eliminar esta entidad?')) return;
  try {
    const res = await fetch(`${API}/entities/${dniRuc}`, { method: 'DELETE' });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error'); }
    showToast('Entidad eliminada');
    const rol = document.getElementById('ent-rol-select').value;
    const zona = document.getElementById('ent-zona-select').value;
    if (rol || zona) searchEntities();
    else document.getElementById('entities-tbody').innerHTML = emptyRow(7);
  } catch (e) { showToast(e.message, true); }
}

document.getElementById('btn-search-products').addEventListener('click', searchProducts);
document.getElementById('btn-show-create-product').addEventListener('click', () => {
  document.getElementById('form-product').reset();
  openModal('modal-product');
});
document.getElementById('form-product').addEventListener('submit', saveProduct);
document.getElementById('btn-close-product-detail').addEventListener('click', () => {
  document.getElementById('product-detail-panel').classList.add('hidden');
});

async function searchProducts() {
  const cat = document.getElementById('prod-cat-select').value;
  const brand = document.getElementById('prod-brand-select').value;
  if (!cat && !brand) return showToast('Seleccione al menos una Categoria o una Marca');

  const tbody = document.getElementById('products-tbody');
  tbody.innerHTML = emptyRow(6, 'Buscando...');

  let params = [];
  if (cat) params.push(`cat=${cat}`);
  if (brand) params.push(`brand=${brand}`);

  try {
    const res = await fetch(`${API}/products?${params.join('&')}`);
    if (!res.ok) throw new Error('Error al consultar productos');
    const data = await res.json();
    if (!data.length) { tbody.innerHTML = emptyRow(6, 'No se encontraron productos.'); return; }

    tbody.innerHTML = '';
    data.forEach(item => {
      const pId = item.productId || (item.pk || '').replace('PROD#', '');
      const estado = item.estado || 'ACTIVO';
      const estadoBadge = estado === 'ACTIVO' ? 'badge-green' : 'badge-red';
      tbody.innerHTML += `<tr>
        <td><strong>${pId}</strong></td>
        <td>${item.nombre || ''}</td>
        <td><span class="badge badge-blue">${item.categoryId || '-'}</span></td>
        <td>${item.brandId || '-'}</td>
        <td><span class="badge ${estadoBadge}">${estado}</span></td>
        <td>
          <button class="btn btn-sm" onclick="viewProductDetail('${pId}')">Ver Detalle</button>
          <button class="btn btn-danger btn-sm" onclick="deleteProduct('${pId}')">Eliminar</button>
        </td>
      </tr>`;
    });
  } catch (e) {
    tbody.innerHTML = emptyRow(6, e.message);
    showToast(e.message, true);
  }
}

let currentViewingProductId = '';
let currentViewingProductName = '';

async function viewProductDetail(productId) {
  currentViewingProductId = productId;
  const panel = document.getElementById('product-detail-panel');
  document.getElementById('pd-title').textContent = productId;
  document.getElementById('pd-info').innerHTML = 'Cargando...';
  document.getElementById('pd-skus-tbody').innerHTML = '';
  document.getElementById('pd-serials-tbody').innerHTML = '';
  panel.classList.remove('hidden');

  try {
    const res = await fetch(`${API}/products/${productId}`);
    if (!res.ok) throw new Error('No se pudo obtener el detalle');
    const data = await res.json();

    currentViewingProductName = data.nombre || productId;
    document.getElementById('pd-title').textContent = currentViewingProductName;
    document.getElementById('pd-info').innerHTML = `
      <p><strong>ID:</strong> ${data.productId}</p>
      <p><strong>Descripcion:</strong> ${data.descripcion || '-'}</p>
      <p><strong>Categoria:</strong> ${data.categoryId || '-'}</p>
      <p><strong>Marca:</strong> ${data.brandId || '-'}</p>
      <p><strong>Estado:</strong> ${data.estado || '-'}</p>
    `;

    const skusTbody = document.getElementById('pd-skus-tbody');
    if (data.variantes && data.variantes.length > 0) {
      skusTbody.innerHTML = '';
      data.variantes.forEach(v => {
        const sk = v.SK || '';
        const skuId = sk.replace('SKU#', '') || v.varianteId || '';
        skusTbody.innerHTML += `<tr>
          <td><strong>${skuId}</strong></td>
          <td>${v.color || '-'}</td>
          <td>${v.ram || '-'}</td>
          <td>${v.precio != null ? 'S/ ' + Number(v.precio).toFixed(2) : '-'}</td>
          <td>${v.stockTotal != null ? v.stockTotal : '-'}</td>
        </tr>`;

        if (skuId && v.precio) {
          registeredSKUs[skuId] = {
            name: `${currentViewingProductName} (${v.color || ''} ${v.ram || ''})`,
            price: v.precio,
            prodId: productId
          };
        }
      });
    } else {
      skusTbody.innerHTML = emptyRow(5, 'Sin variantes registradas.');
    }

    const serialsTbody = document.getElementById('pd-serials-tbody');
    if (data.seriales && data.seriales.length > 0) {
      serialsTbody.innerHTML = '';
      data.seriales.forEach(s => {
        const sk = s.SK || '';
        const sn = sk.replace('SN#', '');
        serialsTbody.innerHTML += `<tr>
          <td>${sn}</td>
          <td><span class="badge ${s.estadoFisico === 'Disponible' ? 'badge-green' : 'badge-red'}">${s.estadoFisico || '-'}</span></td>
          <td>${s.skuAsociado || '-'}</td>
        </tr>`;
      });
    } else {
      serialsTbody.innerHTML = emptyRow(3, 'Sin unidades fisicas registradas.');
    }
  } catch (e) {
    document.getElementById('pd-info').innerHTML = `<p style="color:red;">${e.message}</p>`;
    showToast(e.message, true);
  }
}

document.getElementById('btn-show-create-sku').addEventListener('click', () => {
  if (!currentViewingProductId) return showToast('Seleccione un producto primero', true);
  document.getElementById('form-sku').reset();
  document.getElementById('fsku-productId').value = currentViewingProductId;
  document.getElementById('fsku-productName').value = currentViewingProductName;
  openModal('modal-sku');
});

document.getElementById('form-sku').addEventListener('submit', saveSku);

async function saveSku(e) {
  e.preventDefault();
  const productId = document.getElementById('fsku-productId').value;
  const color = document.getElementById('fsku-color').value.trim();
  const ram = document.getElementById('fsku-ram').value.trim();
  const precio = parseFloat(document.getElementById('fsku-precio').value) || 0;
  const stockTotal = parseInt(document.getElementById('fsku-stockTotal').value) || 0;

  if (!color || !ram) return showToast('Ingrese color y especificación RAM/Capacidad', true);
  if (precio <= 0) return showToast('El precio debe ser mayor a 0', true);

  const body = { color, ram, precio, stockTotal };

  try {
    const res = await fetch(`${API}/products/${productId}/skus`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error al guardar SKU'); }
    showToast('Variante SKU creada exitosamente');
    closeModal('modal-sku');
    viewProductDetail(productId);
  } catch (err) {
    showToast(err.message, true);
  }
}

async function saveProduct(e) {
  e.preventDefault();
  const body = {
    nombre: document.getElementById('fp-nombre').value.trim(),
    descripcion: document.getElementById('fp-descripcion').value.trim(),
    categoryId: document.getElementById('fp-categoryId').value,
    brandId: document.getElementById('fp-brandId').value,
    estado: document.getElementById('fp-estado').value
  };
  try {
    const res = await fetch(`${API}/products`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error'); }
    showToast('Producto creado');
    closeModal('modal-product');
  } catch (e) { showToast(e.message, true); }
}

async function deleteProduct(productId) {
  if (!confirm('¿Eliminar este producto?')) return;
  try {
    const res = await fetch(`${API}/products/${productId}`, { method: 'DELETE' });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error'); }
    showToast('Producto eliminado');
    searchProducts();
  } catch (e) { showToast(e.message, true); }
}

document.getElementById('btn-search-orders').addEventListener('click', searchOrders);
document.getElementById('btn-search-order-sn').addEventListener('click', searchOrderBySN);
document.getElementById('btn-close-order-detail').addEventListener('click', () => {
  document.getElementById('order-detail-panel').classList.add('hidden');
});
document.getElementById('btn-close-sn-trace').addEventListener('click', () => {
  document.getElementById('sn-trace-panel').classList.add('hidden');
});

const registeredSKUs = {
  'S24-NEGRO-256GB': { name: 'Samsung Galaxy S24 Negro 256GB', price: 4599, prodId: '497be248-0a10-47e2-8f72-a936aaee1770' },
  'S24-GRIS-512GB': { name: 'Samsung Galaxy S24 Gris 512GB', price: 5299, prodId: '497be248-0a10-47e2-8f72-a936aaee1770' },
  'MBP16-GRAY-512GB': { name: 'MacBook Pro 16 M3 Space Gray', price: 12999, prodId: '33c2bb8e-125a-4e3b-9be2-35be0bdcfe93' },
  'MBP16-SILVER-1TB': { name: 'MacBook Pro 16 M3 Silver 1TB', price: 15999, prodId: '33c2bb8e-125a-4e3b-9be2-35be0bdcfe93' },
  'MON-DELL-27-4K': { name: 'Monitor Dell 27 4K', price: 1650, prodId: 'PROD_MONITOR_4K' },
  'KEY-LOGI-MX': { name: 'Teclado Logitech MX Keys', price: 320, prodId: 'PROD_TECLADO_MEC' }
};

function onSkuSelected(input) {
  const row = input.closest('.order-item-row');
  if (!row) return;
  const sku = input.value.trim();
  const priceInput = row.querySelector('.oi-precioUnitario');
  if (registeredSKUs[sku]) {
    priceInput.value = registeredSKUs[sku].price;
  }
}

document.getElementById('btn-show-create-order').addEventListener('click', () => {
  document.getElementById('form-order').reset();
  const container = document.getElementById('fo-items-container');
  container.innerHTML = createOrderItemRowHTML(0, false);
  openModal('modal-order');
});

document.getElementById('btn-add-order-item').addEventListener('click', () => {
  const container = document.getElementById('fo-items-container');
  const idx = container.querySelectorAll('.order-item-row').length;
  container.insertAdjacentHTML('beforeend', createOrderItemRowHTML(idx, true));
});

document.getElementById('form-order').addEventListener('submit', saveOrder);

function removeOrderItemRow(btn) {
  const row = btn.closest('.order-item-row');
  if (row) row.remove();
}

function createOrderItemRowHTML(idx, showRemove = false) {
  const removeBtn = showRemove 
    ? `<button type="button" class="btn btn-danger btn-sm" onclick="removeOrderItemRow(this)" style="height:32px; margin-bottom:2px;">Quitar</button>` 
    : '';

  return `<div class="order-item-row" data-idx="${idx}">
    <div style="display:flex; gap:10px; flex-wrap:wrap; margin-bottom:8px; padding:10px; background:#f8fafc; border-radius:4px; border:1px solid #e2e8f0; align-items:flex-end;">
      <label style="flex:2.5; min-width:180px;">Producto / SKU:
        <input type="text" class="oi-skuId" list="sku-datalist" placeholder="Seleccione o escriba SKU" oninput="onSkuSelected(this)" required>
      </label>
      <label style="width:80px;">Cantidad:
        <input type="number" class="oi-cantidad" value="1" min="1" required>
      </label>
      <label style="width:120px;">Precio Unit. (S/):
        <input type="number" class="oi-precioUnitario" step="0.01" value="0" placeholder="0.00" required>
      </label>
      ${removeBtn}
    </div>
  </div>`;
}

async function saveOrder(e) {
  e.preventDefault();
  const userDniRuc = document.getElementById('fo-userDniRuc').value.trim();
  const tipoOperacion = document.getElementById('fo-tipoOperacion').value;
  const estadoPago = document.getElementById('fo-estadoPago').value;

  if (!userDniRuc) return showToast('Ingrese el documento del cliente o proveedor', true);

  const rows = document.querySelectorAll('#fo-items-container .order-item-row');
  const items = [];
  let itemCounter = 1;

  for (const row of rows) {
    const skuId = row.querySelector('.oi-skuId').value.trim();
    const cantidad = parseInt(row.querySelector('.oi-cantidad').value) || 1;
    const precioUnitario = parseFloat(row.querySelector('.oi-precioUnitario').value) || 0;

    if (!skuId) return showToast('Ingrese el SKU del producto', true);
    if (precioUnitario <= 0) return showToast('El precio unitario debe ser mayor a 0', true);

    let productId = 'PROD_' + skuId;
    if (registeredSKUs[skuId]) {
      productId = registeredSKUs[skuId].prodId;
    }

    const serialNumber = `SN-${skuId.substring(0, 8)}-${Math.floor(100 + Math.random() * 900)}`;

    items.push({ productId, skuId, serialNumber, cantidad, precioUnitario });
    itemCounter++;
  }

  if (items.length === 0) return showToast('Agregue al menos un item a la transaccion', true);

  const body = { userDniRuc, tipoOperacion, estadoPago, items };

  try {
    const res = await fetch(`${API}/orders`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    });
    if (!res.ok) { const err = await res.json(); throw new Error(err.error || 'Error al registrar'); }
    showToast('Transaccion registrada exitosamente');
    closeModal('modal-order');
    searchOrders();
  } catch (err) { showToast(err.message, true); }
}

async function searchOrders() {
  const user = document.getElementById('ord-user-input').value.trim();
  const tipo = document.getElementById('ord-filter-type').value;
  const fromDate = document.getElementById('ord-from-date').value;
  const toDate = document.getElementById('ord-to-date').value;

  const tbody = document.getElementById('orders-tbody');
  tbody.innerHTML = emptyRow(7, 'Buscando...');

  let params = [];
  if (user) params.push(`user=${user}`);
  if (tipo) params.push(`tipo=${tipo}`);
  if (fromDate) params.push(`from=${fromDate}`);
  if (toDate) params.push(`to=${toDate}`);

  const url = `${API}/orders` + (params.length > 0 ? `?${params.join('&')}` : '');

  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error('Error al consultar transacciones');
    const data = await res.json();
    if (!data.length) { tbody.innerHTML = emptyRow(7, 'No se encontraron transacciones con los criterios seleccionados.'); return; }

    tbody.innerHTML = '';
    data.forEach(ord => {
      const trxId = ord.trxId || (ord.pk || '').replace('TRX#', '');
      const tipoBadge = ord.tipoOperacion === 'Compra' ? 'badge-blue' : 'badge-green';
      tbody.innerHTML += `<tr>
        <td><strong>${trxId.substring(0, 8)}...</strong></td>
        <td><span class="badge ${tipoBadge}">${ord.tipoOperacion || ''}</span></td>
        <td>${ord.userDniRuc || ''}</td>
        <td>S/ ${Number(ord.total || 0).toFixed(2)}</td>
        <td><span class="badge badge-green">${ord.estadoPago || ''}</span></td>
        <td>${ord.fecha ? ord.fecha.replace('T', ' ').substring(0, 19) : ''}</td>
        <td><button class="btn btn-sm" onclick="viewOrderDetail('${trxId}')">Ver Detalle</button></td>
      </tr>`;
    });
  } catch (e) {
    tbody.innerHTML = emptyRow(7, e.message);
    showToast(e.message, true);
  }
}

async function viewOrderDetail(trxId) {
  const panel = document.getElementById('order-detail-panel');
  document.getElementById('od-title').textContent = trxId.substring(0, 8) + '...';
  document.getElementById('od-info').innerHTML = 'Cargando...';
  document.getElementById('od-items-tbody').innerHTML = '';
  panel.classList.remove('hidden');

  try {
    const res = await fetch(`${API}/orders/${trxId}`);
    if (!res.ok) throw new Error('No se pudo obtener el detalle de la transaccion');
    const data = await res.json();

    const h = data.header || data;
    document.getElementById('od-info').innerHTML = `
      <p><strong>ID Transaccion:</strong> ${h.trxId || trxId}</p>
      <p><strong>Tipo:</strong> ${h.tipoOperacion || '-'}</p>
      <p><strong>Cliente/Proveedor:</strong> ${h.userDniRuc || '-'}</p>
      <p><strong>Total:</strong> S/ ${Number(h.total || 0).toFixed(2)}</p>
      <p><strong>Estado:</strong> ${h.estadoPago || '-'}</p>
      <p><strong>Fecha:</strong> ${h.fecha || '-'}</p>
    `;

    const items = data.detalles || data.details || data.items || [];
    const itemsTbody = document.getElementById('od-items-tbody');
    if (items.length > 0) {
      itemsTbody.innerHTML = '';
      items.forEach(det => {
        itemsTbody.innerHTML += `<tr>
          <td><strong>${det.productId || '-'}</strong></td>
          <td><span class="badge badge-blue">${det.skuId || '-'}</span></td>
          <td>${det.serialNumber ? '<span class="badge badge-green">' + det.serialNumber + '</span>' : '-'}</td>
          <td>${det.cantidad || 1}</td>
          <td>S/ ${Number(det.precioUnitario || 0).toFixed(2)}</td>
          <td><strong>S/ ${Number(det.subtotal || 0).toFixed(2)}</strong></td>
        </tr>`;
      });
    } else {
      itemsTbody.innerHTML = emptyRow(6, 'Sin items de detalle.');
    }
  } catch (e) {
    document.getElementById('od-info').innerHTML = `<p style="color:red;">${e.message}</p>`;
    showToast(e.message, true);
  }
}

async function searchOrderBySN() {
  const sn = document.getElementById('ord-sn-input').value.trim();
  if (!sn) return showToast('Ingrese un numero de serie para rastrear');

  const panel = document.getElementById('sn-trace-panel');
  document.getElementById('sn-trace-title').textContent = sn;
  document.getElementById('sn-trace-results').innerHTML = 'Buscando...';
  panel.classList.remove('hidden');

  try {
    const res = await fetch(`${API}/orders/search?sn=${sn}`);
    if (!res.ok) {
      if (res.status === 404) { document.getElementById('sn-trace-results').innerHTML = '<p>No se encontraron registros para ese numero de serie.</p>'; return; }
      throw new Error('Error al rastrear');
    }
    const data = await res.json();
    const records = data.records || [];

    if (!records.length) {
      document.getElementById('sn-trace-results').innerHTML = '<p>No se encontraron registros asociados.</p>';
      return;
    }

    let html = '<table class="data-table"><thead><tr><th>Producto</th><th>SKU</th><th>Estado</th></tr></thead><tbody>';
    records.forEach(r => {
      html += `<tr>
        <td>${r.productId || (r.PK || '').replace('PROD#', '') || '-'}</td>
        <td>${r.skuAsociado || '-'}</td>
        <td>${r.estadoFisico || '-'}</td>
      </tr>`;
    });
    html += '</tbody></table>';
    document.getElementById('sn-trace-results').innerHTML = html;
  } catch (e) {
    document.getElementById('sn-trace-results').innerHTML = `<p style="color:red;">${e.message}</p>`;
    showToast(e.message, true);
  }
}

document.addEventListener('DOMContentLoaded', () => {
  loadCatalogs();
  document.getElementById('catalogs-tbody').innerHTML = emptyRow(6);
  document.getElementById('entities-tbody').innerHTML = emptyRow(7);
  document.getElementById('products-tbody').innerHTML = emptyRow(6);
  document.getElementById('orders-tbody').innerHTML = emptyRow(7);
});
