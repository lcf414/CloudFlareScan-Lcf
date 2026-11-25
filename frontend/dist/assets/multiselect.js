export const maxExportResults = 500;

function recognizedRegionCode(iataCode) {
  const code = (iataCode || '').trim();
  return /^[A-Za-z]{3}$/.test(code) ? code.toUpperCase() : '';
}

function buildRegionOptions(results) {
  const regions = new Map();
  for (const result of results) {
    const code = recognizedRegionCode(result.iataCode);
    if (!code) continue;
    if (!regions.has(code)) regions.set(code, { name: result.chineseName || code, count: 0 });
    regions.get(code).count++;
  }
  return [...regions].sort(([a], [b]) => a.localeCompare(b))
    .map(([code, { name, count }]) => ({ code, count, display: `${code} (${name}) · ${count} 项` }));
}

export function createCheckMenu({ ref, watch, createVNode: h }) {
  return {
    props: {
      label: { type: String, required: true },
      options: { type: Array, required: true },
      modelValue: { type: Array, required: true },
      disabled: Boolean,
    },
    emits: ['update:modelValue'],
    setup(props, { emit }) {
      const open = ref(false);
      const root = ref(null);
      const trigger = ref(null);
      watch(() => props.disabled, disabled => {
        if (disabled) open.value = false;
      });
      watch(open, (isOpen, _, onCleanup) => {
        if (!isOpen) return;
        const closeOutside = event => {
          if (!root.value?.contains(event.target)) open.value = false;
        };
        document.addEventListener('pointerdown', closeOutside, true);
        onCleanup(() => document.removeEventListener('pointerdown', closeOutside, true));
      });

      function closeOnBlur(event) {
        if (event.relatedTarget && !event.currentTarget.contains(event.relatedTarget)) open.value = false;
      }

      function closeOnEscape(event) {
        if (event.key === 'Escape' && open.value) {
          event.stopPropagation();
          open.value = false;
          trigger.value.focus();
        }
      }

      function checkRow(key, label, checked, onChange, indeterminate = false) {
        return h('label', { key, class: 'cfs-check-row' }, [
          h('input', { type: 'checkbox', checked, indeterminate, onChange }),
          h('span', null, label),
        ]);
      }

      return () => {
        const selected = new Set(props.modelValue);
        const allChecked = selected.size > 0 && selected.size === props.options.length;
        let summary = props.label;
        if (selected.size === 1) summary = String(props.modelValue[0]);
        if (selected.size > 1) summary = allChecked ? '全部' : `${selected.size} 项`;

        return h('div', {
          ref: root,
          class: 'cfs-check-menu',
          onFocusout: closeOnBlur,
          onKeydown: closeOnEscape,
        }, [
          h('button', {
            ref: trigger,
            type: 'button',
            class: 'cfs-check-trigger',
            disabled: props.disabled,
            'aria-label': `${props.label}，${selected.size ? `已选 ${selected.size} 项` : '未选择'}`,
            'aria-expanded': open.value,
            onClick: () => { open.value = !open.value; },
          }, summary),
          open.value ? h('div', { class: 'cfs-check-options', role: 'group', 'aria-label': props.label }, [
            checkRow('all', '全选', allChecked, () => {
              emit('update:modelValue', allChecked ? [] : props.options.map(option => option.code));
            }, selected.size > 0 && !allChecked),
            ...props.options.map(({ code, display }) => checkRow(code, display, selected.has(code), event => {
              if (event.target.checked) selected.add(code);
              else selected.delete(code);
              emit('update:modelValue', props.options.map(option => option.code).filter(value => selected.has(value)));
            })),
          ]) : null,
        ]);
      };
    },
  };
}

export function createRegionChecklist({ createVNode: h }) {
  return {
    props: {
      label: { type: String, required: true },
      options: { type: Array, required: true },
      modelValue: { type: Array, required: true },
      disabled: Boolean,
    },
    emits: ['update:modelValue'],
    setup(props, { emit }) {
      function checkbox(key, text, checked, onChange, indeterminate = false) {
        return h('label', { key, class: 'cfs-inline-region' }, [
          h('input', { type: 'checkbox', checked, indeterminate, disabled: props.disabled, onChange }),
          h('span', null, text),
        ]);
      }

      return () => {
        if (!props.options.length) return null;
        const checked = new Set(props.modelValue);
        const codes = props.options.map(option => option.code);
        const all = codes.every(code => checked.has(code));
        return h('div', { class: 'cfs-region-checklist', role: 'group', 'aria-label': props.label }, [
          checkbox('all', '全选', all, () => emit('update:modelValue', all ? [] : codes), !all && checked.size > 0),
          ...props.options.map(({ code, display }) => checkbox(code, display, checked.has(code), event => {
            if (event.target.checked) checked.add(code);
            else checked.delete(code);
            emit('update:modelValue', codes.filter(value => checked.has(value)));
          })),
        ]);
      };
    },
  };
}

export function createExportDialog({ ref, watch, createVNode: h }, RegionChecklist) {
  return {
    props: { visible: Boolean, regions: { type: Array, required: true } },
    emits: ['close', 'confirm'],
    setup(props, { emit }) {
      const dialog = ref(null);
      const format = ref('csv');
      const layout = ref('default');
      const selectedRegions = ref([]);
      watch(() => props.visible, visible => {
        if (visible) {
          format.value = 'csv';
          layout.value = 'default';
          selectedRegions.value = props.regions.map(region => region.code);
          dialog.value.showModal();
        } else if (dialog.value.open) {
          dialog.value.close();
        }
      });

      function option(name, value, label, selection) {
        return h('label', { class: 'cfs-export-option' }, [
          h('input', { type: 'radio', name, value, checked: selection.value === value,
            onChange: () => { selection.value = value; } }),
          h('span', null, label),
        ]);
      }

      return () => h('dialog', {
        ref: dialog, class: 'cfs-export-dialog', 'aria-labelledby': 'cfs-export-title',
        onCancel: event => { event.preventDefault(); emit('close'); },
      }, [h('form', {
        onSubmit: event => {
          event.preventDefault();
          if (selectedRegions.value.length) emit('confirm', format.value, layout.value, selectedRegions.value);
        },
      }, [
        h('h2', { id: 'cfs-export-title' }, '导出结果'),
        h('div', { class: 'cfs-export-row', role: 'group', 'aria-label': '文件格式' }, [
          h('span', null, '格式'),
          option('cfs-export-format', 'csv', 'CSV', format),
          option('cfs-export-format', 'txt', 'TXT', format),
        ]),
        h('div', { class: 'cfs-export-row', role: 'group', 'aria-label': '内容样式' }, [
          h('span', null, '样式'),
          option('cfs-export-layout', 'default', '默认', layout),
          option('cfs-export-layout', 'compact', '精简', layout),
        ]),
        h('div', { class: 'cfs-export-regions' }, [
          h('span', null, '地区'),
          h(RegionChecklist, {
            label: '导出地区', options: props.regions, modelValue: selectedRegions.value,
            'onUpdate:modelValue': regions => { selectedRegions.value = regions; },
          }),
        ]),
        h('div', { class: 'cfs-export-actions' }, [
          h('button', { type: 'button', onClick: () => emit('close') }, '取消'),
          h('button', { type: 'submit', class: 'cfs-export-submit', disabled: !selectedRegions.value.length }, '导出'),
        ]),
      ])]);
    },
  };
}

export function createScanActions({ refs, settings, app, on, off, showDialog, appendLog, clearLogs, nextTick }) {
  const {
    busy, scanResults, speedResults, activeTab, selectedRegions, selectedPorts,
    regionOptions, speedCount, progressPercent, statusText, speedText, checkingRoute, routeState, routeRefreshing,
    exportVisible, exporting, exportRegionOptions,
  } = refs;
  let task = null;
  let manualRegionalCount = null;
  const hasRegion = result => Boolean(recognizedRegionCode(result.iataCode));

  function regionalSpeedLimit() {
    const regions = new Set(selectedRegions.value);
    return regionOptions.value.reduce((sum, option) => sum + (regions.has(option.code) ? option.count : 0), 0);
  }

  function setSpeedCount(value) {
    speedCount.value = value;
    const count = Number(value);
    if (selectedRegions.value.length && Number.isInteger(count) && count > 0 && count <= regionalSpeedLimit()) {
      manualRegionalCount = count;
    }
  }

  function changeRegionSelection(regions) {
    if (!selectedRegions.value.length && regions.length) {
      manualRegionalCount = null;
    }
    selectedRegions.value = regions;
    const limit = regionalSpeedLimit();
    speedCount.value = limit ? Math.min(manualRegionalCount ?? limit, limit) : '';
  }

  function begin(kind, status, counters, handlers) {
    const events = { 'log-message': appendLog, ...handlers };
    const current = { kind, stopping: false, events: Object.keys(events) };
    task = current;
    busy.value = true;
    progressPercent.value = 0;
    statusText.value = status;
    speedText.value = counters;
    for (const [event, handler] of Object.entries(events)) {
      off(event);
      on(event, (...args) => { if (task === current) handler(...args); });
    }
    return current;
  }

  function finish(current, status) {
    if (task !== current) return;
    for (const event of current.events) off(event);
    task = null;
    busy.value = false;
    statusText.value = current.stopping ? '任务已停止' : status;
  }

  function fail(current, error) {
    if (task !== current) return;
    const message = error?.message || String(error);
    appendLog(message);
    finish(current, '就绪');
    showDialog(message, 'alert');
  }

  function describeRoutes(routes) {
    return routes.map(({ version, state, device }) => {
      const label = version ? `IPv${version}` : '出站路径';
      const status = state === 'direct' ? '直连网卡' :
        state === 'tunnel' ? '检测到 TUN/VPN 隧道' : '无法确认是否直连';
      return `${label}：${status}${device ? `（${device}）` : ''}`;
    }).join('\n');
  }

  async function readRoutes(check) {
    try {
      const routes = await check();
      if (Array.isArray(routes) && routes.length) return routes;
    } catch {
      // A failed route query cannot establish direct access.
    }
    return [{ state: 'unknown' }];
  }

  async function refreshRouteStatus() {
    if (routeRefreshing.value) return;
    routeRefreshing.value = true;
    try {
      const routes = await readRoutes(() => app().CheckDirectRoute(0));
      if (routes.some(route => route.state === 'tunnel')) routeState.value = 'tunnel';
      else if (routes.some(route => route.state === 'direct')) routeState.value = 'direct';
      else routeState.value = 'unknown';
    } finally {
      routeRefreshing.value = false;
    }
  }

  async function runWithRoute(check, start) {
    if (busy.value || checkingRoute.value) return;
    checkingRoute.value = true;
    void refreshRouteStatus();
    const routes = await readRoutes(check);
    checkingRoute.value = false;
    if (routes.some(route => route.state === 'tunnel')) {
      showDialog(`${describeRoutes(routes)}\n请关闭 TUN/VPN 或切换为直连后重试。`, 'alert');
      return;
    }
    if (routes.some(route => route.state !== 'direct')) {
      showDialog(`${describeRoutes(routes)}\n仍要继续本次任务吗？`, 'confirm', start);
      return;
    }
    await start();
  }

  async function startScan(version) {
    if (busy.value || checkingRoute.value) return;
    const ports = [...selectedPorts.value];
    if (!ports.length) {
      showDialog('请至少勾选一个端口', 'alert');
      return;
    }
    await runWithRoute(() => app().CheckDirectRoute(version), () => runScan(version, ports));
  }

  async function runScan(version, ports) {
    if (busy.value) return;
    const previousScan = scanResults.value;
    const previousSpeed = speedResults.value;
    const previousOptions = regionOptions.value;
    clearLogs();
    speedResults.value = [];
    scanResults.value = [];
    regionOptions.value = [];
    activeTab.value = 'log';
    const current = begin('scan', `IPv${version}扫描中...`, '速度: 0 项/秒', {
      'scan-progress': (done, total, found, rate) => {
        if (total > 0) progressPercent.value = done / total * 100;
        if (!current.stopping) statusText.value = `扫描中: ${done}/${total} (${found} 项可用)`;
        speedText.value = `速度: ${rate.toFixed(1)} 项/秒`;
      },
      'scan-completed': returned => {
        const results = returned || [];
        scanResults.value = results;
        const portCounts = new Map(ports.map(port => [port, 0]));
        for (const result of results) {
          portCounts.set(result.port, (portCounts.get(result.port) || 0) + 1);
        }
        regionOptions.value = buildRegionOptions(results);
        if (selectedRegions.value.length) {
          const available = new Set(regionOptions.value.map(option => option.code));
          changeRegionSelection(selectedRegions.value.filter(code => available.has(code)));
        }
        appendLog(`可用地址与端口组合共 ${results.length} 项，识别到 ${regionOptions.value.length} 个地区。`);
        for (const [port, count] of portCounts) appendLog(`端口 ${port}：${count} 项`);
        speedText.value = '速度: 0 项/秒';
        finish(current, '扫描完成');
      },
    });
    try {
      await app().StartMultiPortScan(version, ports, settings.maxWorkers, settings.latencyThreshold,
        settings.v4Sample, settings.v6Sample);
    } catch (error) {
      if (task === current) {
        scanResults.value = previousScan;
        speedResults.value = previousSpeed;
        regionOptions.value = previousOptions;
      }
      fail(current, error);
    }
  }

  async function startSpeedTest(regional) {
    if (busy.value || checkingRoute.value || !scanResults.value.length) return;
    let regions = [];
    let count = 0;
    if (regional) {
      regions = [...selectedRegions.value];
      if (!regions.length) {
        showDialog('请至少勾选一个地区', 'alert');
        return;
      }
      count = Number(speedCount.value);
      const limit = regionalSpeedLimit();
      if (!Number.isInteger(count) || count < 1 || count > limit) {
        showDialog(`测速数量范围：1-${limit}`, 'alert');
        return;
      }
    }
    await runWithRoute(() => app().CheckSpeedTestRoute(regions, count), () => runSpeedTest(regional, regions, count));
  }

  async function runSpeedTest(regional, regions, count) {
    if (busy.value) return;
    const previousSpeed = speedResults.value;
    speedResults.value = [];
    const current = begin('speed', regional ? '地区测速中...' : '完全测速中...', '测速进度: 0/0', {
      'speed-progress': (done, total) => {
        if (total > 0) progressPercent.value = done / total * 100;
        if (!current.stopping) statusText.value = `测速中: ${done}/${total}`;
        speedText.value = `测速进度: ${done}/${total}`;
      },
      'speed-completed': returned => {
        speedResults.value = returned || [];
        if (speedResults.value.length) activeTab.value = 'speed';
        appendLog(`测速结果共 ${speedResults.value.length} 项。`);
        finish(current, '测速完成');
      },
    });
    try {
      if (regional) await app().StartMultiRegionSpeedTest(regions, count);
      else await app().StartAllScannedSpeedTest();
    } catch (error) {
      if (task === current) speedResults.value = previousSpeed;
      fail(current, error);
    }
  }

  function openExport() {
    if (busy.value || checkingRoute.value || exporting.value) return;
    const results = speedResults.value.some(hasRegion) ? speedResults.value : scanResults.value;
    exportRegionOptions.value = buildRegionOptions(results);
    if (!exportRegionOptions.value.length) {
      showDialog('没有识别到地区的结果可导出', 'alert');
      return;
    }
    exportVisible.value = true;
  }

  async function exportResults(format, layout, regions) {
    if (!exportVisible.value || exporting.value || !regions.length) return;
    exportVisible.value = false;
    exporting.value = true;
    const selected = new Set(regions);
    const selectedCount = exportRegionOptions.value.reduce((sum, option) => sum + (selected.has(option.code) ? option.count : 0), 0);
    const count = Math.min(selectedCount, maxExportResults);
    const kind = speedResults.value.some(hasRegion) ? '测速' : '扫描';
    try {
      await nextTick();
      const path = await app().ExportCurrentRegionResults(format, layout, regions);
      if (path) {
        appendLog(`已导出 ${count} 条${kind}结果：${path}`);
        statusText.value = '结果已导出';
      }
    } catch (error) {
      showDialog(`导出失败：${error?.message || String(error)}`, 'alert');
    } finally {
      exporting.value = false;
    }
  }

  function showStopDialog() {
    const current = task;
    if (!current || current.stopping) return;
    showDialog('确定要停止当前任务吗？', 'confirm', async () => {
      if (task !== current) return;
      current.stopping = true;
      statusText.value = '正在停止任务...';
      try {
        if (current.kind === 'scan') await app().StopScan();
        else await app().StopSpeedTest();
      } catch (error) {
        if (task !== current) return;
        current.stopping = false;
        statusText.value = '任务执行中...';
        appendLog(String(error));
      }
    });
  }

  return {
    startScan, startFullSpeed: () => startSpeedTest(false), startRegionSpeed: () => startSpeedTest(true), showStopDialog,
    refreshRouteStatus,
    changeRegionSelection, regionalSpeedLimit, setSpeedCount, openExport, exportResults,
    closeExport: () => { exportVisible.value = false; },
  };
}
