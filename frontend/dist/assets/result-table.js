const columns = ['类型', '国家', '地区', 'IP', '端口', '延迟', '测速结果'];
const pageSize = 100;

function addressKey(result) {
  return `${result.ip}|${result.port}`;
}

function codeFor(result) {
  return (result.iataCode || '').trim().toUpperCase();
}

export function createResultTable({ ref, watch, computed, createVNode: h }) {
  return {
    props: {
      scanResults: { type: Array, required: true },
      speedResults: { type: Array, required: true },
      regions: { type: Array, required: true },
      selectedRegions: { type: Array, required: true },
      measuredOnly: Boolean,
    },
    emits: ['copy'],
    setup(props, { emit }) {
      const page = ref(1);
      const scroller = ref(null);
      const rows = computed(() => {
        const regions = new Set(props.selectedRegions.length ? props.selectedRegions : props.regions.map(region => region.code));
        const matchesRegion = result => regions.has(codeFor(result));
        if (props.measuredOnly) {
          if (props.speedResults[0]?.testType === '完全测速') return props.speedResults;
          return props.speedResults.filter(matchesRegion);
        }

        const speeds = new Map(props.speedResults.map(result => [addressKey(result), result.downloadSpeed]));
        return props.scanResults.filter(matchesRegion)
          .sort((a, b) => a.latencyMs - b.latencyMs)
          .map(result => ({ ...result, downloadSpeed: speeds.get(addressKey(result)) }));
      });

      function showPage(number) {
        page.value = number;
        if (scroller.value) scroller.value.scrollTop = 0;
      }

      watch(rows, () => showPage(1));

      function renderRow(result, index) {
        const type = result.ip.includes(':') ? 'IPv6' : 'IPv4';
        const region = codeFor(result);
        const latency = Number.isFinite(result.latencyMs) ? `${result.latencyMs.toFixed(2)} ms` : '';
        const speed = Number.isFinite(result.downloadSpeed) ? `${result.downloadSpeed.toFixed(2)} MB/s` : '';
        return h('tr', { key: `${addressKey(result)}|${index}` }, [
          h('td', null, type),
          h('td', null, result.countryCode || ''),
          h('td', null, region === 'UNKNOWN' ? '' : region),
          h('td', { class: 'cfs-table-ip' }, [
            h('button', { type: 'button', title: '复制 IP', onClick: () => emit('copy', result.ip) }, result.ip),
          ]),
          h('td', null, String(result.port)),
          h('td', null, latency),
          h('td', null, speed),
        ]);
      }

      return () => {
        const pages = Math.max(1, Math.ceil(rows.value.length / pageSize));
        const offset = (page.value - 1) * pageSize;
        const pageRows = rows.value.slice(offset, offset + pageSize);
        return h('section', { class: 'cfs-results-panel' }, [
          h('div', { class: 'cfs-table-scroll', ref: scroller }, [
            h('table', { class: 'cfs-result-table', 'aria-label': props.measuredOnly ? '测速 IP 列表' : '筛选出的 IP 列表' }, [
              h('thead', null, [h('tr', null, columns.map(column => h('th', { key: column, scope: 'col' }, column)))]),
              h('tbody', null, pageRows.length ? pageRows.map((result, index) => renderRow(result, offset + index)) : [
                h('tr', null, [h('td', { colspan: columns.length, class: 'cfs-table-empty' }, '暂无符合条件的 IP')]),
              ]),
            ]),
          ]),
          pages > 1 ? h('nav', { class: 'cfs-table-pages', 'aria-label': 'IP 列表分页' }, [
            h('button', { type: 'button', disabled: page.value <= 1, onClick: () => showPage(page.value - 1) }, '上一页'),
            h('span', { 'aria-live': 'polite' }, `${page.value} / ${pages} 页 · 共 ${rows.value.length} 条`),
            h('button', { type: 'button', disabled: page.value >= pages, onClick: () => showPage(page.value + 1) }, '下一页'),
          ]) : null,
        ]);
      };
    },
  };
}
