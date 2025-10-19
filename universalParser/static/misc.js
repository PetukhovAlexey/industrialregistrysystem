const CompanyManager = {
    // Текущий пользователь
    currentUser: {
        id: 1,
        name: 'Алексей Петухов',
        role: 'admin',
        email: 'alexey@company.ru'
    },

    // Данные компаний (загружаем из localStorage или создаем пустой массив)
    companies: JSON.parse(localStorage.getItem('companies')) || [],
    
    // Приглашения (инвайты)
    invites: JSON.parse(localStorage.getItem('invites')) || [],

    // Текущая выбранная компания для документов
    currentCompanyId: 'all',

    // Настройки пагинации
    currentPage: 1,
    companiesPerPage: 12,

    // Данные для графиков
    companiesData: [],

    // Переменные для хранения данных и графиков аналитики
    allCompanies: [],
    charts: {},

    // Фильтры для аналитики
    filters: JSON.parse(localStorage.getItem('analyticsFilters')) || [],
    currentFilter: null,

    // Инициализация приложения
    init() {
        this.checkPermissions();
        this.initFileUpload();
        this.loadDocuments();
        this.loadCompanies();
        this.initCharts();
        this.initToast();
        this.setupEventListeners();
        this.updateCompanySelects();
        this.generateSampleData();
        this.initAdditionalCharts();
        this.initAnalytics();
        initUploadForm();
        
        // Инициализация менеджера редактирования данных
        DataEditingManager.init();
    },

    // Инициализация аналитики
    initAnalytics() {
        // Загружаем фильтры из JSON
        this.loadFiltersFromJSON();
        
        // Обработчики для фильтров
        document.getElementById('saveFilterBtn').addEventListener('click', () => {
            this.saveFilter();
        });

        document.getElementById('downloadChartsBtn').addEventListener('click', () => {
            this.downloadChartsAsHTML();
        });
    },

    // Загрузка фильтров из JSON файла
    async loadFiltersFromJSON() {
        try {
            const response = await fetch('/static/filters/filters.json');
            if (!response.ok) {
                throw new Error('Файл filters.json не найден');
            }
            this.filters = await response.json();
            localStorage.setItem('analyticsFilters', JSON.stringify(this.filters));
            this.loadFiltersCombobox();
        } catch (error) {
            console.error('Ошибка загрузки filters.json:', error);
            // Если файл не загружен, используем демо-фильтры
            this.filters = [
                {
                    "name": "filter1",
                    "query": "prom1.json",
                    "uri": "/static/filters/prom1.json"
                },
                {
                    "name": "filter2", 
                    "query": "prom2.json",
                    "uri": "/static/filters/prom2.json"
                }
            ];
            this.loadFiltersCombobox();
        }
    },

    // Загрузка данных по выбранному фильтру
    async loadDataByFilter(filterIndex) {
        const filter = this.filters[filterIndex];
        if (!filter || !filter.uri) {
            alert('У выбранного фильтра нет URI для загрузки данных');
            return;
        }

        try {
            // Показываем индикатор загрузки
            this.showLoadingIndicator();
            
            const response = await fetch(filter.uri);
            if (!response.ok) {
                throw new Error(`Ошибка загрузки данных: ${response.status}`);
            }
            
            const data = await response.json();
            this.processCompaniesData(data);
            this.showToast(`Данные загружены по фильтру: ${filter.name}`);
            
        } catch (error) {
            console.error('Ошибка загрузки данных:', error);
            alert(`Ошибка загрузки данных: ${error.message}`);
        } finally {
            this.hideLoadingIndicator();
        }
    },

    // Показать индикатор загрузки
    showLoadingIndicator() {
        const fileUploadArea = document.getElementById('fileUploadArea');
        fileUploadArea.innerHTML = `
            <div class="file-upload-content text-center">
                <div class="spinner-border text-primary mb-3" role="status">
                    <span class="visually-hidden">Загрузка...</span>
                </div>
                <h4>Загрузка данных...</h4>
                <p class="text-muted">Пожалуйста, подождите</p>
            </div>
        `;
    },

    // Скрыть индикатор загрузки
    hideLoadingIndicator() {
        // Восстанавливаем оригинальное содержимое
        const fileUploadArea = document.getElementById('fileUploadArea');
        fileUploadArea.innerHTML = `
            <div class="file-upload-content">
                <i class="bi bi-funnel display-4 text-muted mb-3"></i>
                <h4>Выберите фильтр для загрузки данных</h4>
                <p class="text-muted mb-3">Используйте комбобокс выше для выбора фильтра и загрузки соответствующих данных</p>
                <div class="alert alert-info">
                    <i class="bi bi-info-circle me-2"></i>
                    Данные будут автоматически загружены из указанного в фильтре источника
                </div>
            </div>
        `;
    },

    // Загрузка фильтров в комбобокс
    loadFiltersCombobox() {
        const filtersDropdownMenu = document.getElementById('filtersDropdownMenu');
        const selectedFilterText = document.getElementById('selectedFilterText');

        // Очищаем меню
        filtersDropdownMenu.innerHTML = '';

        if (this.filters.length === 0) {
            // Если фильтров нет
            filtersDropdownMenu.innerHTML = `
                <li>
                    <div class="filter-option">
                        <div class="d-flex align-items-center">
                            <i class="bi bi-exclamation-triangle text-warning me-2"></i>
                            <div>
                                <div class="filter-name">Фильтры не найдены</div>
                                <div class="filter-description">Нет доступных фильтров для загрузки</div>
                            </div>
                        </div>
                    </div>
                </li>
            `;
            selectedFilterText.textContent = 'Нет фильтров';
            return;
        }

        // Добавляем существующие фильтры
        this.filters.forEach((filter, index) => {
            const filterElement = document.createElement('li');
            filterElement.innerHTML = `
                <div class="filter-option" onclick="CompanyManager.selectFilter(${index})">
                    <div class="d-flex align-items-start">
                        <i class="bi bi-funnel text-primary me-2 mt-1"></i>
                        <div class="flex-grow-1">
                            <div class="filter-name">
                                <span class="jira-like-name">${filter.name}</span>
                            </div>
                            <div class="filter-description">${filter.query || 'Без описания'}</div>
                            <div class="small text-muted mt-1">${filter.uri}</div>
                        </div>
                    </div>
                </div>
            `;
            filtersDropdownMenu.appendChild(filterElement);
        });

        // Устанавливаем текст выбранного фильтра
        if (this.currentFilter !== null) {
            selectedFilterText.textContent = this.filters[this.currentFilter].name;
        } else {
            selectedFilterText.textContent = 'Выберите фильтр';
        }
    },

    // Выбор фильтра
    selectFilter(filterIndex) {
        this.currentFilter = filterIndex;
        const filter = this.filters[filterIndex];
        
        // Обновляем текст в комбобоксе
        document.getElementById('selectedFilterText').textContent = filter.name;
        
        // Закрываем dropdown
        const dropdown = bootstrap.Dropdown.getInstance(document.getElementById('filtersDropdown'));
        dropdown.hide();
        
        // Загружаем данные по выбранному фильтру
        this.loadDataByFilter(filterIndex);
    },

    // Обработка данных компаний - собираем все объекты с id
    processCompaniesData(data) {
        this.allCompanies = [];
        
        // Проходим по всем ключам (названиям компаний) и собираем все объекты с id
        for (const companyGroup in data) {
            const companyArray = data[companyGroup];
            if (Array.isArray(companyArray)) {
                companyArray.forEach(company => {
                    if (company.id) {
                        // Добавляем название группы для идентификации
                        company.companyGroup = companyGroup;
                        this.allCompanies.push(company);
                    }
                });
            }
        }
        
        // Сортируем по id для удобства
        this.allCompanies.sort((a, b) => a.id - b.id);
        
        // Скрываем область загрузки и показываем статистику и графики
        document.getElementById('fileUploadArea').classList.add('hidden');
        document.getElementById('statsContainer').classList.remove('hidden');
        document.getElementById('chartsContainer').classList.remove('hidden');
        
        // Обновляем статистику
        this.updateStatistics();
        
        // Строим графики
        this.createCharts();
    },

    // Обновление статистики
    updateStatistics() {
        let totalRevenue = 0;
        let totalEmployees = 0;
        let totalProfit = 0;
        
        this.allCompanies.forEach(company => {
            totalRevenue += company.revenue || 0;
            totalEmployees += company.total_staff || 0;
            totalProfit += company.net_profit || 0;
        });
        
        // Обновляем DOM
        document.getElementById('totalCompanies').textContent = this.allCompanies.length;
        document.getElementById('totalRevenue').textContent = (totalRevenue / 1000000000).toFixed(2);
        document.getElementById('totalEmployees').textContent = totalEmployees.toLocaleString();
        document.getElementById('totalProfit').textContent = (totalProfit / 1000000).toFixed(0);
    },

    // Сохранение фильтра
    saveFilter() {
        const name = document.getElementById('filterName').value;
        const description = document.getElementById('filterDescription').value;
        const query = document.getElementById('filterQuery').value;

        if (!name || !description || !query) {
            alert('Пожалуйста, заполните все поля');
            return;
        }

        const newFilter = {
            id: Date.now(),
            name: name,
            description: description,
            query: query,
            createdAt: new Date().toISOString()
        };

        this.filters.push(newFilter);
        localStorage.setItem('analyticsFilters', JSON.stringify(this.filters));
        
        // Обновляем комбобокс
        this.loadFiltersCombobox();
        
        // Закрываем модальное окно
        const modal = bootstrap.Modal.getInstance(document.getElementById('addFilterModal'));
        modal.hide();
        
        // Сбрасываем форму
        document.getElementById('addFilterForm').reset();
        
        this.showToast('Фильтр успешно сохранен!');
    },

    // Скачивание графиков как HTML
    downloadChartsAsHTML() {
        if (this.allCompanies.length === 0) {
            alert('Нет данных для экспорта. Сначала загрузите данные через фильтр.');
            return;
        }

        // Создаем HTML-строку с графиками
        const htmlContent = this.generateHTMLReport();
        
        // Создаем blob и скачиваем
        const blob = new Blob([htmlContent], { type: 'text/html' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `analytics-report-${new Date().toISOString().split('T')[0]}.html`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    },

    // Генерация HTML отчета
    generateHTMLReport() {
        const currentDate = new Date().toLocaleDateString('ru-RU');
        
        return `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Аналитический отчет - ${currentDate}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"><\/script>
    <style>
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; 
            margin: 0; 
            padding: 20px; 
            background-color: #f8f9fa; 
        }
        .header { 
            text-align: center; 
            margin-bottom: 30px; 
            background: white;
            padding: 30px;
            border-radius: 12px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .stats { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); 
            gap: 15px; 
            margin-bottom: 30px; 
        }
        .stat-card { 
            background: white; 
            padding: 20px; 
            border-radius: 8px; 
            text-align: center; 
            box-shadow: 0 2px 4px rgba(0,0,0,0.1); 
        }
        .stat-number { 
            font-size: 24px; 
            font-weight: bold; 
            color: #2c5aa0; 
        }
        .stat-label { 
            font-size: 14px; 
            color: #6c757d; 
            margin-top: 5px; 
        }
        .chart-section {
            margin-bottom: 40px;
        }
        .chart-row {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 20px;
            margin-bottom: 20px;
        }
        .chart-container { 
            background: white; 
            padding: 20px; 
            border-radius: 8px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1); 
        }
        .chart-full {
            grid-column: 1 / -1;
        }
        canvas { 
            max-width: 100%; 
            height: 300px !important;
        }
        h2 {
            color: #2c5aa0;
            border-bottom: 2px solid #2c5aa0;
            padding-bottom: 10px;
            margin-top: 40px;
        }
        h3 {
            color: #495057;
            margin-bottom: 15px;
        }
        @media (max-width: 768px) {
            .chart-row {
                grid-template-columns: 1fr;
            }
        }
    <\/style>
</head>
<body>
    <div class="header">
        <h1>Аналитический отчет BusinessHub</h1>
        <p><strong>Дата генерации:</strong> ${currentDate}</p>
        <p><strong>Количество компаний:</strong> ${this.allCompanies.length}</p>
        <p><strong>Общая выручка:</strong> ${(this.allCompanies.reduce((sum, c) => sum + (c.revenue || 0), 0) / 1000000000).toFixed(2)} млрд руб.</p>
    </div>
    
    <div class="stats">
        <div class="stat-card">
            <div class="stat-number">${this.allCompanies.length}</div>
            <div class="stat-label">Всего компаний</div>
        </div>
        <div class="stat-card">
            <div class="stat-number">${(this.allCompanies.reduce((sum, c) => sum + (c.revenue || 0), 0) / 1000000000).toFixed(2)}</div>
            <div class="stat-label">Общая выручка (млрд руб.)</div>
        </div>
        <div class="stat-card">
            <div class="stat-number">${this.allCompanies.reduce((sum, c) => sum + (c.total_staff || 0), 0).toLocaleString()}</div>
            <div class="stat-label">Всего сотрудников</div>
        </div>
        <div class="stat-card">
            <div class="stat-number">${(this.allCompanies.reduce((sum, c) => sum + (c.net_profit || 0), 0) / 1000000).toFixed(0)}</div>
            <div class="stat-label">Общая прибыль (млн руб.)</div>
        </div>
    </div>

    <h2>Основные финансовые показатели</h2>
    <div class="chart-row">
        <div class="chart-container">
            <h3>Выручка компаний (млн руб.)</h3>
            <canvas id="revenueChart"></canvas>
        </div>
        <div class="chart-container">
            <h3>Чистая прибыль компаний (млн руб.)</h3>
            <canvas id="profitChart"></canvas>
        </div>
    </div>

    <h2>Налоги и зарплаты</h2>
    <div class="chart-row">
        <div class="chart-container">
            <h3>Общие налоги компаний (млн руб.)</h3>
            <canvas id="taxesChart"></canvas>
        </div>
        <div class="chart-container">
            <h3>Средняя зарплата (руб.)</h3>
            <canvas id="salaryChart"></canvas>
        </div>
    </div>

    <h2>Сотрудники и активы</h2>
    <div class="chart-row">
        <div class="chart-container">
            <h3>Количество сотрудников</h3>
            <canvas id="employeesChart"></canvas>
        </div>
        <div class="chart-container">
            <h3>Общие активы компаний (млн руб.)</h3>
            <canvas id="assetsChart"></canvas>
        </div>
    </div>

    <h2>Производство и экспорт</h2>
    <div class="chart-row">
        <div class="chart-container">
            <h3>Загрузка производственных мощностей (%)</h3>
            <canvas id="utilizationChart"></canvas>
        </div>
        <div class="chart-container">
            <h3>Объем экспорта (млн руб.)</h3>
            <canvas id="exportChart"></canvas>
        </div>
    </div>

    <h2>Анализ распределения</h2>
    <div class="chart-row">
        <div class="chart-container">
            <h3>Распределение выручки по компаниям</h3>
            <canvas id="revenuePieChart"></canvas>
        </div>
        <div class="chart-container">
            <h3>Сравнение выручки и прибыли</h3>
            <canvas id="revenueProfitComparisonChart"></canvas>
        </div>
    </div>

    <h2>Сравнительный анализ</h2>
    <div class="chart-row">
        <div class="chart-container chart-full">
            <h3>Сравнение показателей компаний (радарная диаграмма)</h3>
            <canvas id="radarChart" height="400"></canvas>
        </div>
    </div>

    <script>
        // Данные для графиков
        const companiesData = ${JSON.stringify(this.allCompanies)};
        const companyLabels = companiesData.map(c => \`\${c.companyGroup} (ID: \${c.id})\`);
        
        // Функция для создания цветов
        function generateColors(count) {
            const colors = [
                '#2c5aa0', '#27ae60', '#e74c3c', '#f39c12', '#9b59b6',
                '#34495e', '#1abc9c', '#e67e22', '#3498db', '#2ecc71',
                '#8e44ad', '#d35400', '#c0392b', '#16a085', '#2980b9'
            ];
            return colors.slice(0, count);
        }

        // График выручки
        new Chart(document.getElementById('revenueChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Выручка (млн руб.)',
                    data: companiesData.map(c => (c.revenue || 0) / 1000000),
                    backgroundColor: '#2c5aa0'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    }
                }
            }
        });
        
        // График прибыли
        new Chart(document.getElementById('profitChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Чистая прибыль (млн руб.)',
                    data: companiesData.map(c => (c.net_profit || 0) / 1000000),
                    backgroundColor: '#27ae60'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'млн руб.'
                    }
                }
            }
        });
    }
});

        // График налогов
        new Chart(document.getElementById('taxesChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Общие налоги (млн руб.)',
                    data: companiesData.map(c => (c.total_taxes || 0) / 1000000),
                    backgroundColor: '#e74c3c'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    }
                }
            }
        });

        // График зарплат
        new Chart(document.getElementById('salaryChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Средняя зарплата (руб.)',
                    data: companiesData.map(c => c.avg_salary_total || 0),
                    backgroundColor: '#f39c12'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'руб.'
                        }
                    }
                }
            }
        });

        // График сотрудников
        new Chart(document.getElementById('employeesChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Количество сотрудников',
                    data: companiesData.map(c => c.total_staff || 0),
                    backgroundColor: '#9b59b6'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'человек'
                        }
                    }
                }
            }
        });

        // График активов
        new Chart(document.getElementById('assetsChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Общие активы (млн руб.)',
                    data: companiesData.map(c => (c.total_assets || 0) / 1000000),
                    backgroundColor: '#34495e'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    }
                }
            }
        });

        // График загрузки мощностей
        new Chart(document.getElementById('utilizationChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Загрузка мощностей (%)',
                    data: companiesData.map(c => c.production_capacity_utilization || 0),
                    backgroundColor: '#1abc9c'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        max: 100,
                        title: {
                            display: true,
                            text: '%'
                        }
                    }
                }
            }
        });

        // График экспорта
        new Chart(document.getElementById('exportChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Объем экспорта (млн руб.)',
                    data: companiesData.map(c => (c.export_volume || 0) / 1000000),
                    backgroundColor: '#e67e22'
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    }
                }
            }
        });

        // Круговая диаграмма выручки
        new Chart(document.getElementById('revenuePieChart').getContext('2d'), {
            type: 'pie',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Выручка (млн руб.)',
                    data: companiesData.map(c => (c.revenue || 0) / 1000000),
                    backgroundColor: generateColors(companiesData.length)
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        position: 'right'
                    }
                }
            }
        });

        // Сравнение выручки и прибыли
        new Chart(document.getElementById('revenueProfitComparisonChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [
                    {
                        label: 'Выручка (млн руб.)',
                        data: companiesData.map(c => (c.revenue || 0) / 1000000),
                        backgroundColor: 'rgba(44, 90, 160, 0.7)'
                    },
                    {
                        label: 'Прибыль (млн руб.)',
                        data: companiesData.map(c => (c.net_profit || 0) / 1000000),
                        backgroundColor: 'rgba(25, 135, 84, 0.7)'
                    }
                ]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Млн руб.'
                        }
                    }
                }
            }
        });

        // Радарная диаграмма
        const selectedCompanies = companiesData.slice(0, Math.min(3, companiesData.length));
        const radarLabels = ['Выручка', 'Прибыль', 'Налоги', 'Сотрудники', 'Активы', 'Экспорт'];
        
        const maxRevenue = Math.max(...companiesData.map(c => c.revenue || 0));
        const maxProfit = Math.max(...companiesData.map(c => c.net_profit || 0));
        const maxTaxes = Math.max(...companiesData.map(c => c.total_taxes || 0));
        const maxEmployees = Math.max(...companiesData.map(c => c.total_staff || 0));
        const maxAssets = Math.max(...companiesData.map(c => c.total_assets || 0));
        const maxExport = Math.max(...companiesData.map(c => c.export_volume || 0));

        const radarDatasets = selectedCompanies.map((company, index) => {
            const colors = [
                'rgba(44, 90, 160, 0.5)',
                'rgba(25, 135, 84, 0.5)',
                'rgba(220, 53, 69, 0.5)'
            ];
            
            const borderColors = [
                'rgba(44, 90, 160, 1)',
                'rgba(25, 135, 84, 1)',
                'rgba(220, 53, 69, 1)'
            ];
            
            return {
                label: \`\${company.companyGroup} (ID: \${company.id})\`,
                data: [
                    ((company.revenue || 0) / maxRevenue) * 100,
                    ((company.net_profit || 0) / maxProfit) * 100,
                    ((company.total_taxes || 0) / maxTaxes) * 100,
                    ((company.total_staff || 0) / maxEmployees) * 100,
                    ((company.total_assets || 0) / maxAssets) * 100,
                    ((company.export_volume || 0) / maxExport) * 100
                ],
                backgroundColor: colors[index],
                borderColor: borderColors[index],
                borderWidth: 2
            };
        });

        new Chart(document.getElementById('radarChart').getContext('2d'), {
            type: 'radar',
            data: {
                labels: radarLabels,
                datasets: radarDatasets
            },
            options: {
                responsive: true,
                scales: {
                    r: {
                        beginAtZero: true,
                        max: 100
                    }
                }
            }
        });
    <\/script>
</body>
</html>`;
    },

    // Создание графиков
    createCharts() {
        // Уничтожаем предыдущие графики
        Object.values(this.charts).forEach(chart => {
            if (chart) chart.destroy();
        });
        this.charts = {};
        
        // Получаем данные для графиков
        const companyLabels = this.allCompanies.map(company => 
            `${company.companyGroup} (ID: ${company.id})`
        );
        
        // График выручки
        const revenueCtx = document.getElementById('revenueChart').getContext('2d');
        this.charts.revenue = new Chart(revenueCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Выручка (млн руб.)',
                    data: this.allCompanies.map(company => (company.revenue || 0) / 1000000),
                    backgroundColor: '#2c5aa0',
                    borderColor: '#1e4a8a',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
        
        // График прибыли
        const profitCtx = document.getElementById('profitChart').getContext('2d');
        this.charts.profit = new Chart(profitCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Чистая прибыль (млн руб.)',
                    data: this.allCompanies.map(company => (company.net_profit || 0) / 1000000),
                    backgroundColor: '#27ae60',
                    borderColor: '#219653',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
        
        // График налогов
        const taxesCtx = document.getElementById('taxesChart').getContext('2d');
        this.charts.taxes = new Chart(taxesCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Общие налоги (млн руб.)',
                    data: this.allCompanies.map(company => (company.total_taxes || 0) / 1000000),
                    backgroundColor: '#e74c3c',
                    borderColor: '#c0392b',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
        
        // График зарплат
        const salaryCtx = document.getElementById('salaryChart').getContext('2d');
        this.charts.salary = new Chart(salaryCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Средняя зарплата (руб.)',
                    data: this.allCompanies.map(company => company.avg_salary_total || 0),
                    backgroundColor: '#f39c12',
                    borderColor: '#e67e22',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'руб.'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
        
        // График сотрудников
        const employeesCtx = document.getElementById('employeesChart').getContext('2d');
        this.charts.employees = new Chart(employeesCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Количество сотрудников',
                    data: this.allCompanies.map(company => company.total_staff || 0),
                    backgroundColor: '#9b59b6',
                    borderColor: '#8e44ad',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'человек'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
        
        // График активов
        const assetsCtx = document.getElementById('assetsChart').getContext('2d');
        this.charts.assets = new Chart(assetsCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Общие активы (млн руб.)',
                    data: this.allCompanies.map(company => (company.total_assets || 0) / 1000000),
                    backgroundColor: '#34495e',
                    borderColor: '#2c3e50',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
        
        // График загрузки мощностей
        const utilizationCtx = document.getElementById('utilizationChart').getContext('2d');
        this.charts.utilization = new Chart(utilizationCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Загрузка мощностей (%)',
                    data: this.allCompanies.map(company => company.production_capacity_utilization || 0),
                    backgroundColor: '#1abc9c',
                    borderColor: '#16a085',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        max: 100,
                        title: {
                            display: true,
                            text: '%'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
        
        // График экспорта
        const exportCtx = document.getElementById('exportChart').getContext('2d');
        this.charts.export = new Chart(exportCtx, {
            type: 'bar',
            data: {
                labels: companyLabels,
                datasets: [{
                    label: 'Объем экспорта (млн руб.)',
                    data: this.allCompanies.map(company => (company.export_volume || 0) / 1000000),
                    backgroundColor: '#e67e22',
                    borderColor: '#d35400',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'млн руб.'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });

        // Дополнительные графики
        this.createRevenuePieChart();
        this.createRevenueProfitComparisonChart();
        this.createRadarChart();
    },

    // Новые типы диаграмм
    createRevenuePieChart() {
        const ctx = document.getElementById('revenuePieChart').getContext('2d');
        const labels = this.allCompanies.map(company => 
            `${company.companyGroup} (ID: ${company.id})`
        );
        const data = this.allCompanies.map(company => (company.revenue || 0) / 1000000);

        if (this.charts.revenuePieChart) {
            this.charts.revenuePieChart.destroy();
        }

        this.charts.revenuePieChart = new Chart(ctx, {
            type: 'pie',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Выручка (млн руб.)',
                    data: data,
                    backgroundColor: [
                        '#2c5aa0', '#27ae60', '#e74c3c', '#f39c12', '#9b59b6',
                        '#34495e', '#1abc9c', '#e67e22', '#3498db', '#2ecc71'
                    ],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right'
                    }
                }
            }
        });
    },

    createRevenueProfitComparisonChart() {
        const ctx = document.getElementById('revenueProfitComparisonChart').getContext('2d');
        const labels = this.allCompanies.map(company => 
            `${company.companyGroup} (ID: ${company.id})`
        );
        const revenueData = this.allCompanies.map(company => (company.revenue || 0) / 1000000);
        const profitData = this.allCompanies.map(company => (company.net_profit || 0) / 1000000);

        if (this.charts.revenueProfitComparisonChart) {
            this.charts.revenueProfitComparisonChart.destroy();
        }

        this.charts.revenueProfitComparisonChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Выручка (млн руб.)',
                        data: revenueData,
                        backgroundColor: 'rgba(44, 90, 160, 0.7)',
                        borderColor: 'rgba(44, 90, 160, 1)',
                        borderWidth: 1
                    },
                    {
                        label: 'Прибыль (млн руб.)',
                        data: profitData,
                        backgroundColor: 'rgba(25, 135, 84, 0.7)',
                        borderColor: 'rgba(25, 135, 84, 1)',
                        borderWidth: 1
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Млн руб.'
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45
                        }
                    }
                }
            }
        });
    },

    createRadarChart() {
        const ctx = document.getElementById('radarChart').getContext('2d');
        
        // Выбираем первые 3 компании для сравнения
        const selectedCompanies = this.allCompanies.slice(0, Math.min(3, this.allCompanies.length));
        const labels = ['Выручка', 'Прибыль', 'Налоги', 'Сотрудники', 'Активы', 'Экспорт'];
        
        const datasets = selectedCompanies.map((company, index) => {
            const colors = [
                'rgba(44, 90, 160, 0.5)',
                'rgba(25, 135, 84, 0.5)',
                'rgba(220, 53, 69, 0.5)'
            ];
            
            const borderColors = [
                'rgba(44, 90, 160, 1)',
                'rgba(25, 135, 84, 1)',
                'rgba(220, 53, 69, 1)'
            ];
            
            // Нормализуем данные для радарной диаграммы
            const maxRevenue = Math.max(...this.allCompanies.map(c => c.revenue || 0));
            const maxProfit = Math.max(...this.allCompanies.map(c => c.net_profit || 0));
            const maxTaxes = Math.max(...this.allCompanies.map(c => c.total_taxes || 0));
            const maxEmployees = Math.max(...this.allCompanies.map(c => c.total_staff || 0));
            const maxAssets = Math.max(...this.allCompanies.map(c => c.total_assets || 0));
            const maxExport = Math.max(...this.allCompanies.map(c => c.export_volume || 0));
            
            return {
                label: `${company.companyGroup} (ID: ${company.id})`,
                data: [
                    ((company.revenue || 0) / maxRevenue) * 100,
                    ((company.net_profit || 0) / maxProfit) * 100,
                    ((company.total_taxes || 0) / maxTaxes) * 100,
                    ((company.total_staff || 0) / maxEmployees) * 100,
                    ((company.total_assets || 0) / maxAssets) * 100,
                    ((company.export_volume || 0) / maxExport) * 100
                ],
                backgroundColor: colors[index],
                borderColor: borderColors[index],
                borderWidth: 2,
                pointBackgroundColor: borderColors[index]
            };
        });

        if (this.charts.radarChart) {
            this.charts.radarChart.destroy();
        }

        this.charts.radarChart = new Chart(ctx, {
            type: 'radar',
            data: {
                labels: labels,
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    r: {
                        beginAtZero: true,
                        max: 100
                    }
                }
            }
        });
    },

    // Инициализация дополнительных графиков
    initAdditionalCharts() {
        // Этот метод можно использовать для инициализации дополнительных графиков
        // при загрузке демонстрационных данных
    },

    // Генерация демонстрационных данных
    generateSampleData() {
        // Если данных нет, создаем демонстрационные данные
        if (this.companies.length === 0) {
            const sampleCompanies = [
                {
                    id: 1,
                    name: 'ТехноПром Групп',
                    inn: '1234567890',
                    revenue: 150.5,
                    growth: 12.3,
                    profit: 25.8,
                    createdAt: new Date().toISOString()
                },
                {
                    id: 2,
                    name: 'СтройИнвест Холдинг',
                    inn: '0987654321',
                    revenue: 89.2,
                    growth: 8.7,
                    profit: 12.4,
                    createdAt: new Date().toISOString()
                }
            ];
            this.companies = sampleCompanies;
            this.saveCompanies();
        }
    },

    // Остальные методы управления компаниями
    checkPermissions() {
        // Проверка прав доступа
        if (this.currentUser.role !== 'admin') {
            console.warn('У пользователя ограниченные права доступа');
        }
    },

    initFileUpload() {
        // Инициализация загрузки файлов
        const fileInput = document.getElementById('documentFile');
        if (fileInput) {
            fileInput.addEventListener('change', function(e) {
                const fileName = e.target.files[0]?.name || 'Файл не выбран';
                const label = this.previousElementSibling;
                if (label && label.classList.contains('form-label')) {
                    label.textContent = `Выбран файл: ${fileName}`;
                }
            });
        }
    },

    loadDocuments() {
        // Загрузка документов
        const documents = JSON.parse(localStorage.getItem('documents')) || [];
        this.renderDocuments(documents);
    },

    renderDocuments(documents) {
        // Рендеринг документов
        const container = document.getElementById('documents-container');
        if (!container) return;

        if (documents.length === 0) {
            container.innerHTML = `
                <div class="col-12 text-center py-4">
                    <i class="bi bi-folder-x display-4 text-muted mb-3"></i>
                    <h5 class="text-muted">Документов пока нет</h5>
                    <p class="text-muted">Добавьте первый документ, чтобы начать работу</p>
                </div>
            `;
            return;
        }

        container.innerHTML = documents.map(doc => `
            <div class="col-md-6 col-lg-4 mb-3">
                <div class="card h-100">
                    <div class="card-body">
                        <div class="d-flex align-items-start mb-2">
                            <i class="bi bi-file-earmark-text text-primary fs-4 me-3"></i>
                            <div class="flex-grow-1">
                                <h6 class="card-title mb-1">${doc.name}</h6>
                                <small class="text-muted">${doc.company}</small>
                            </div>
                        </div>
                        <p class="card-text small text-muted">${doc.description || 'Описание отсутствует'}</p>
                        <div class="d-flex justify-content-between align-items-center">
                            <small class="text-muted">${new Date(doc.date).toLocaleDateString()}</small>
                            <div>
                                <button class="btn btn-sm btn-outline-primary me-1">
                                    <i class="bi bi-download"></i>
                                </button>
                                <button class="btn btn-sm btn-outline-danger">
                                    <i class="bi bi-trash"></i>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `).join('');
    },

    loadCompanies() {
        // Загрузка компаний
        this.renderCompanies();
        this.updateStats();
    },

    renderCompanies() {
        // Рендеринг компаний
        const container = document.getElementById('companies-charts-container');
        const emptyMessage = document.getElementById('empty-companies-message');

        if (this.companies.length === 0) {
            if (container) container.style.display = 'none';
            if (emptyMessage) emptyMessage.style.display = 'block';
            return;
        }

        if (container) container.style.display = 'block';
        if (emptyMessage) emptyMessage.style.display = 'none';

        // Пагинация
        const startIndex = (this.currentPage - 1) * this.companiesPerPage;
        const endIndex = startIndex + this.companiesPerPage;
        const companiesToShow = this.companies.slice(startIndex, endIndex);

        // Рендеринг компаний
        if (container) {
            container.innerHTML = companiesToShow.map(company => `
                <div class="col-xl-3 col-lg-4 col-md-6 mb-4">
                    <div class="card company-chart-card shadow-sm h-100">
                        <div class="card-header bg-white d-flex justify-content-between align-items-center">
                            <h6 class="mb-0">${company.name}</h6>
                            <div class="dropdown">
                                <button class="btn btn-sm btn-outline-secondary dropdown-toggle" type="button" 
                                        data-bs-toggle="dropdown">
                                    <i class="bi bi-gear"></i>
                                </button>
                                <ul class="dropdown-menu">
                                    <li><a class="dropdown-item" href="#" onclick="CompanyManager.editCompany(${company.id})">
                                        <i class="bi bi-pencil me-2"></i>Редактировать
                                    </a></li>
                                    <li><a class="dropdown-item" href="#" onclick="CompanyManager.viewDetails(${company.id})">
                                        <i class="bi bi-eye me-2"></i>Просмотр
                                    </a></li>
                                    <li><hr class="dropdown-divider"></li>
                                    <li><a class="dropdown-item text-danger" href="#" onclick="CompanyManager.deleteCompany(${company.id})">
                                        <i class="bi bi-trash me-2"></i>Удалить
                                    </a></li>
                                </ul>
                            </div>
                        </div>
                        <div class="card-body">
                            <div class="company-chart-container">
                                <canvas id="chart-${company.id}"></canvas>
                            </div>
                            <div class="company-mini-stats">
                                <div class="row text-center">
                                    <div class="col-4">
                                        <small class="text-muted d-block">Выручка</small>
                                        <strong class="text-success">${company.revenue}M</strong>
                                    </div>
                                    <div class="col-4">
                                        <small class="text-muted d-block">Рост</small>
                                        <strong class="text-primary">${company.growth}%</strong>
                                    </div>
                                    <div class="col-4">
                                        <small class="text-muted d-block">Прибыль</small>
                                        <strong class="text-info">${company.profit}M</strong>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            `).join('');

            // Инициализация графиков для компаний
            companiesToShow.forEach(company => {
                this.initCompanyChart(company);
            });
        }

        // Обновление пагинации
        this.updatePagination();
    },

    initCompanyChart(company) {
        // Инициализация графика для компании
        const ctx = document.getElementById(`chart-${company.id}`)?.getContext('2d');
        if (!ctx) return;

        // Создаем случайные данные для демонстрации
        const months = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн'];
        const revenueData = months.map(() => Math.floor(Math.random() * 100) + 50);
        const profitData = months.map(() => Math.floor(Math.random() * 30) + 10);

        new Chart(ctx, {
            type: 'line',
            data: {
                labels: months,
                datasets: [
                    {
                        label: 'Выручка',
                        data: revenueData,
                        borderColor: '#2c5aa0',
                        backgroundColor: 'rgba(44, 90, 160, 0.1)',
                        tension: 0.4,
                        fill: true
                    },
                    {
                        label: 'Прибыль',
                        data: profitData,
                        borderColor: '#198754',
                        backgroundColor: 'rgba(25, 135, 84, 0.1)',
                        tension: 0.4,
                        fill: true
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        display: false
                    },
                    x: {
                        display: false
                    }
                }
            }
        });
    },

    updatePagination() {
        // Обновление пагинации
        const totalPages = Math.ceil(this.companies.length / this.companiesPerPage);
        const paginationContainer = document.getElementById('companies-tab-pagination');
        
        if (!paginationContainer) return;

        let paginationHTML = '';

        // Кнопка "Назад"
        paginationHTML += `
            <li class="page-item ${this.currentPage === 1 ? 'disabled' : ''}">
                <a class="page-link" href="#" onclick="CompanyManager.changePage(${this.currentPage - 1})">Назад</a>
            </li>
        `;

        // Номера страниц
        for (let i = 1; i <= totalPages; i++) {
            paginationHTML += `
                <li class="page-item ${i === this.currentPage ? 'active' : ''}">
                    <a class="page-link" href="#" onclick="CompanyManager.changePage(${i})">${i}</a>
                </li>
            `;
        }

        // Кнопка "Вперед"
        paginationHTML += `
            <li class="page-item ${this.currentPage === totalPages ? 'disabled' : ''}">
                <a class="page-link" href="#" onclick="CompanyManager.changePage(${this.currentPage + 1})">Вперед</a>
            </li>
        `;

        paginationContainer.innerHTML = paginationHTML;
    },

    changePage(page) {
        // Смена страницы
        const totalPages = Math.ceil(this.companies.length / this.companiesPerPage);
        if (page < 1 || page > totalPages) return;
        
        this.currentPage = page;
        this.renderCompanies();
    },

    updateStats() {
        // Обновление статистики
        const companiesCount = this.companies.length;
        const totalRevenue = this.companies.reduce((sum, company) => sum + parseFloat(company.revenue), 0);
        const avgGrowth = this.companies.length > 0 
            ? this.companies.reduce((sum, company) => sum + parseFloat(company.growth), 0) / this.companies.length 
            : 0;

        document.getElementById('companies-count').textContent = companiesCount;
        document.getElementById('total-revenue-count').textContent = `${totalRevenue.toFixed(1)}M`;
        document.getElementById('avg-growth-count').textContent = `${avgGrowth.toFixed(1)}%`;
    },

    updateCompanySelects() {
        // Обновление выпадающих списков компаний
        const selects = document.querySelectorAll('select[id="company-select"], select[name="company"]');
        selects.forEach(select => {
            const currentValue = select.value;
            select.innerHTML = '<option value="all">Все компании</option>' +
                this.companies.map(company => 
                    `<option value="${company.id}" ${company.id == currentValue ? 'selected' : ''}>${company.name}</option>`
                ).join('');
        });
    },

    initCharts() {
        // Инициализация основных графиков
        // Может быть использована для глобальных графиков на дашборде
    },

    initToast() {
        // Инициализация уведомлений
        this.toast = new bootstrap.Toast(document.getElementById('inviteToast'));
    },

    setupEventListeners() {
        // Настройка обработчиков событий
        
        // Создание компании
        const createCompanyForm = document.getElementById('createCompanyForm');
        if (createCompanyForm) {
            createCompanyForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.createCompany(new FormData(createCompanyForm));
            });
        }

        // Загрузка документа
        const uploadForm = document.getElementById('uploadForm');
        if (uploadForm) {
            uploadForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.uploadDocument(new FormData(uploadForm));
            });
        }

        // Смена компании в фильтре документов
        const companySelect = document.getElementById('company-select');
        if (companySelect) {
            companySelect.addEventListener('change', (e) => {
                this.currentCompanyId = e.target.value;
                this.filterDocuments();
            });
        }
    },

    createCompany(formData) {
        // Создание новой компании
        const company = {
            id: Date.now(),
            name: formData.get('companyName'),
            inn: formData.get('inn'),
            kpp: formData.get('kpp'),
            ogrn: formData.get('ogrn'),
            legalAddress: formData.get('legalAddress'),
            actualAddress: formData.get('actualAddress'),
            phone: formData.get('phone'),
            email: formData.get('email'),
            account: formData.get('account'),
            bank: formData.get('bank'),
            bik: formData.get('bik'),
            correspondentAccount: formData.get('correspondentAccount'),
            revenue: parseFloat(formData.get('revenue')),
            growth: parseFloat(formData.get('growth')),
            profit: parseFloat(formData.get('profit')),
            createdAt: new Date().toISOString()
        };

        this.companies.push(company);
        this.saveCompanies();
        this.renderCompanies();
        this.updateStats();
        this.updateCompanySelects();

        // Закрываем модальное окно
        const modal = bootstrap.Modal.getInstance(document.getElementById('createCompanyModal'));
        modal.hide();

        // Сбрасываем форму
        document.getElementById('createCompanyForm').reset();

        // Показываем уведомление
        this.showToast('Компания успешно создана!');
    },

    uploadDocument(formData) {
        // Загрузка документа
        const document = {
            id: Date.now(),
            name: formData.get('documentFile').name,
            company: formData.get('company'),
            description: formData.get('description') || 'Без описания',
            date: new Date().toISOString(),
            size: Math.floor(Math.random() * 5000) + 100 // Случайный размер для демо
        };

        const documents = JSON.parse(localStorage.getItem('documents')) || [];
        documents.push(document);
        localStorage.setItem('documents', JSON.stringify(documents));

        this.loadDocuments();

        // Закрываем модальное окно
        const modal = bootstrap.Modal.getInstance(document.getElementById('uploadModal'));
        modal.hide();

        // Сбрасываем форму
        document.getElementById('uploadForm').reset();

        this.showToast('Документ успешно загружен!');
    },

    filterDocuments() {
        // Фильтрация документов по компании
        const documents = JSON.parse(localStorage.getItem('documents')) || [];
        let filteredDocuments = documents;

        if (this.currentCompanyId !== 'all') {
            filteredDocuments = documents.filter(doc => doc.company === this.currentCompanyId);
        }

        this.renderDocuments(filteredDocuments);
    },

    editCompany(companyId) {
        // Редактирование компании
        const company = this.companies.find(c => c.id === companyId);
        if (company) {
            // Заполняем форму редактирования
            // В реальном приложении здесь будет открытие модального окна редактирования
            this.showToast(`Редактирование компании: ${company.name}`);
        }
    },

    viewDetails(companyId) {
        // Просмотр деталей компании
        const company = this.companies.find(c => c.id === companyId);
        if (company) {
            // В реальном приложении здесь будет переход на страницу деталей
            this.showToast(`Просмотр деталей компании: ${company.name}`);
        }
    },

    deleteCompany(companyId) {
        // Удаление компании
        if (confirm('Вы уверены, что хотите удалить эту компанию?')) {
            this.companies = this.companies.filter(c => c.id !== companyId);
            this.saveCompanies();
            this.renderCompanies();
            this.updateStats();
            this.updateCompanySelects();
            this.showToast('Компания успешно удалена!');
        }
    },

    generateAdminLink() {
        // Генерация ссылки для приглашения администратора
        const linkInput = document.getElementById('admin-invite-link');
        const randomToken = Math.random().toString(36).substring(2, 15);
        const inviteLink = `${window.location.origin}/invite/${randomToken}`;
        
        linkInput.value = inviteLink;
        
        // Сохраняем инвайт
        const invite = {
            token: randomToken,
            companyId: null, // Будет установлено при создании компании
            expires: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
            used: false
        };
        
        this.invites.push(invite);
        localStorage.setItem('invites', JSON.stringify(this.invites));
        
        this.showToast('Ссылка для приглашения сгенерирована!');
    },

    copyAdminLink() {
        // Копирование ссылки для приглашения
        const linkInput = document.getElementById('admin-invite-link');
        if (linkInput.value) {
            linkInput.select();
            document.execCommand('copy');
            this.showToast('Ссылка скопирована в буфер обмена!');
        }
    },

    showToast(message) {
        // Показ уведомления
        const toastMessage = document.getElementById('toastMessage');
        if (toastMessage) {
            toastMessage.textContent = message;
        }
        if (this.toast) {
            this.toast.show();
        }
    },

    saveCompanies() {
        // Сохранение компаний в localStorage
        localStorage.setItem('companies', JSON.stringify(this.companies));
    }
};

// Модуль для управления редактированием данных
const DataEditingManager = {
    // Текущие данные для редактирования
    currentData: [],
    // Измененные данные
    modifiedData: new Map(),
    // Текущая страница
    currentPage: 1,
    // Количество строк на странице
    rowsPerPage: 25,
    // Индикатор сохранения
    saveToast: null,

    // Инициализация модуля
    init() {
        this.setupEventListeners();
        this.initSaveToast();
        this.loadDataFromAnalytics();
    },

    // Настройка обработчиков событий
    setupEventListeners() {
        // Кнопка сохранения изменений
        const saveChangesBtn = document.getElementById('saveChangesBtn');
        const refreshDataBtn = document.getElementById('refreshDataBtn');
        const exportDataBtn = document.getElementById('exportDataBtn');
        const rowsPerPageSelect = document.getElementById('rowsPerPageSelect');
        const dataEditingTab = document.querySelector('a[data-bs-target="#data-editing-tab"]');

        if (saveChangesBtn) {
            saveChangesBtn.addEventListener('click', () => {
                this.saveChanges();
            });
        }

        if (refreshDataBtn) {
            refreshDataBtn.addEventListener('click', () => {
                this.loadDataFromAnalytics();
            });
        }

        if (exportDataBtn) {
            exportDataBtn.addEventListener('click', () => {
                this.exportData();
            });
        }

        if (rowsPerPageSelect) {
            rowsPerPageSelect.addEventListener('change', (e) => {
                this.rowsPerPage = parseInt(e.target.value);
                this.currentPage = 1;
                this.renderTable();
            });
        }

        if (dataEditingTab) {
            dataEditingTab.addEventListener('click', () => {
                this.loadDataFromAnalytics();
            });
        }
    },

    // Инициализация тоста для уведомлений о сохранении
    initSaveToast() {
        const saveToastElement = document.getElementById('saveToast');
        if (saveToastElement) {
            this.saveToast = new bootstrap.Toast(saveToastElement);
        }
    },

    // Загрузка данных из модуля аналитики
    loadDataFromAnalytics() {
        const analyticsData = CompanyManager.allCompanies;
        
        if (analyticsData && analyticsData.length > 0) {
            this.currentData = analyticsData;
            this.modifiedData.clear();
            this.renderTable();
            this.updateUI();
        } else {
            this.showNoDataMessage();
        }
    },

    // Отображение таблицы с данными
    renderTable() {
        const tableBody = document.getElementById('editableDataTableBody');
        const noDataMessage = document.getElementById('noDataMessage');
        const tableContainer = document.querySelector('.table-container');
        const pagination = document.getElementById('editingPagination');

        if (!tableBody || !noDataMessage || !tableContainer || !pagination) return;

        if (this.currentData.length === 0) {
            tableBody.innerHTML = '';
            noDataMessage.style.display = 'block';
            tableContainer.style.display = 'none';
            pagination.style.display = 'none';
            return;
        }

        noDataMessage.style.display = 'none';
        tableContainer.style.display = 'block';
        pagination.style.display = 'flex';

        // Расчет пагинации
        const startIndex = (this.currentPage - 1) * this.rowsPerPage;
        const endIndex = Math.min(startIndex + this.rowsPerPage, this.currentData.length);
        const pageData = this.currentData.slice(startIndex, endIndex);

        // Генерация строк таблицы
        tableBody.innerHTML = pageData.map((company, index) => {
            const globalIndex = startIndex + index;
            const isModified = this.modifiedData.has(globalIndex);
            
            return `
                <tr data-index="${globalIndex}">
                    <td>${company.id}</td>
                    <td>${company.companyGroup}</td>
                    <td class="editable-cell" data-field="revenue">${this.formatCellValue((company.revenue || 0) / 1000000, isModified, globalIndex, 'revenue')}</td>
                    <td class="editable-cell" data-field="net_profit">${this.formatCellValue((company.net_profit || 0) / 1000000, isModified, globalIndex, 'net_profit')}</td>
                    <td class="editable-cell" data-field="total_taxes">${this.formatCellValue((company.total_taxes || 0) / 1000000, isModified, globalIndex, 'total_taxes')}</td>
                    <td class="editable-cell" data-field="total_staff">${this.formatCellValue(company.total_staff || 0, isModified, globalIndex, 'total_staff')}</td>
                    <td class="editable-cell" data-field="total_assets">${this.formatCellValue((company.total_assets || 0) / 1000000, isModified, globalIndex, 'total_assets')}</td>
                    <td class="editable-cell" data-field="avg_salary_total">${this.formatCellValue(company.avg_salary_total || 0, isModified, globalIndex, 'avg_salary_total')}</td>
                    <td class="editable-cell" data-field="production_capacity_utilization">${this.formatCellValue(company.production_capacity_utilization || 0, isModified, globalIndex, 'production_capacity_utilization')}</td>
                    <td class="editable-cell" data-field="export_volume">${this.formatCellValue((company.export_volume || 0) / 1000000, isModified, globalIndex, 'export_volume')}</td>
                    <td class="action-buttons">
                        <button class="btn btn-sm btn-outline-primary" onclick="DataEditingManager.editRow(${globalIndex})" title="Редактировать всю строку">
                            <i class="bi bi-pencil"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger" onclick="DataEditingManager.resetRow(${globalIndex})" title="Сбросить изменения">
                            <i class="bi bi-arrow-counterclockwise"></i>
                        </button>
                    </td>
                </tr>
            `;
        }).join('');

        // Настройка обработчиков для редактируемых ячеек
        this.setupCellEditors();
        
        // Обновление пагинации
        this.updatePagination();
    },

    // Форматирование значения ячейки
    formatCellValue(value, isModified, index, field) {
        const modifiedValue = this.getModifiedValue(index, field);
        const displayValue = modifiedValue !== undefined ? modifiedValue : value;
        const cellClass = isModified ? 'modified-cell' : '';
        
        if (typeof displayValue === 'number') {
            return `<span class="${cellClass}">${displayValue.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>`;
        }
        
        return `<span class="${cellClass}">${displayValue}</span>`;
    },

    // Настройка редакторов ячеек
    setupCellEditors() {
        const editableCells = document.querySelectorAll('.editable-cell');
        
        editableCells.forEach(cell => {
            cell.addEventListener('click', (e) => {
                if (!e.target.classList.contains('editable-input')) {
                    this.startEditing(cell);
                }
            });
        });
    },

    // Начало редактирования ячейки
    startEditing(cell) {
        const rowIndex = parseInt(cell.closest('tr').dataset.index);
        const field = cell.dataset.field;
        const currentValue = this.getCurrentValue(rowIndex, field);
        
        cell.classList.add('editing');
        cell.innerHTML = `<input type="number" class="editable-input" value="${currentValue}" step="0.01">`;
        
        const input = cell.querySelector('.editable-input');
        input.focus();
        input.select();
        
        // Обработчики для завершения редактирования
        const finishEditing = () => {
            const newValue = parseFloat(input.value);
            if (!isNaN(newValue) && newValue !== currentValue) {
                this.setModifiedValue(rowIndex, field, newValue);
            }
            this.renderTable();
        };
        
        input.addEventListener('blur', finishEditing);
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                finishEditing();
            } else if (e.key === 'Escape') {
                this.renderTable();
            }
        });
    },

    // Получение текущего значения (оригинального или измененного)
    getCurrentValue(rowIndex, field) {
        const modifiedValue = this.getModifiedValue(rowIndex, field);
        if (modifiedValue !== undefined) {
            return modifiedValue;
        }
        
        const company = this.currentData[rowIndex];
        const value = company[field] || 0;
        
        // Конвертация в соответствующие единицы измерения
        if (field === 'revenue' || field === 'net_profit' || field === 'total_taxes' || 
            field === 'total_assets' || field === 'export_volume') {
            return value / 1000000;
        }
        
        return value;
    },

    // Получение измененного значения
    getModifiedValue(rowIndex, field) {
        const modifiedRow = this.modifiedData.get(rowIndex);
        return modifiedRow ? modifiedRow[field] : undefined;
    },

    // Установка измененного значения
    setModifiedValue(rowIndex, field, value) {
        let modifiedRow = this.modifiedData.get(rowIndex);
        if (!modifiedRow) {
            modifiedRow = {};
            this.modifiedData.set(rowIndex, modifiedRow);
        }
        modifiedRow[field] = value;
        this.updateUI();
    },

    // Редактирование всей строки
    editRow(rowIndex) {
        const company = this.currentData[rowIndex];
        let modifiedRow = this.modifiedData.get(rowIndex) || {};
        
        // В реальном приложении здесь будет модальное окно для редактирования
        // Для демонстрации просто запросим новые значения через prompt
        const newRevenue = prompt('Введите новое значение выручки (млн руб.):', 
            (modifiedRow.revenue !== undefined ? modifiedRow.revenue : (company.revenue || 0) / 1000000));
        
        if (newRevenue !== null) {
            const revenueValue = parseFloat(newRevenue);
            if (!isNaN(revenueValue)) {
                this.setModifiedValue(rowIndex, 'revenue', revenueValue);
                this.renderTable();
            }
        }
    },

    // Сброс изменений в строке
    resetRow(rowIndex) {
        if (this.modifiedData.has(rowIndex)) {
            this.modifiedData.delete(rowIndex);
            this.renderTable();
            this.updateUI();
            this.showToast('Изменения в строке сброшены', 'info');
        }
    },

    // Обновление пагинации
    updatePagination() {
        const totalPages = Math.ceil(this.currentData.length / this.rowsPerPage);
        const paginationContainer = document.getElementById('editingPagination');
        
        if (!paginationContainer) return;

        if (totalPages <= 1) {
            paginationContainer.innerHTML = '';
            return;
        }

        let paginationHTML = '';

        // Кнопка "Назад"
        paginationHTML += `
            <li class="page-item ${this.currentPage === 1 ? 'disabled' : ''}">
                <a class="page-link" href="#" onclick="DataEditingManager.changePage(${this.currentPage - 1})">Назад</a>
            </li>
        `;

        // Номера страниц
        for (let i = 1; i <= totalPages; i++) {
            paginationHTML += `
                <li class="page-item ${i === this.currentPage ? 'active' : ''}">
                    <a class="page-link" href="#" onclick="DataEditingManager.changePage(${i})">${i}</a>
                </li>
            `;
        }

        // Кнопка "Вперед"
        paginationHTML += `
            <li class="page-item ${this.currentPage === totalPages ? 'disabled' : ''}">
                <a class="page-link" href="#" onclick="DataEditingManager.changePage(${this.currentPage + 1})">Вперед</a>
            </li>
        `;

        paginationContainer.innerHTML = paginationHTML;
    },

    // Смена страницы
    changePage(page) {
        const totalPages = Math.ceil(this.currentData.length / this.rowsPerPage);
        if (page < 1 || page > totalPages) return;
        
        this.currentPage = page;
        this.renderTable();
    },

    // Обновление интерфейса
    updateUI() {
        const saveButton = document.getElementById('saveChangesBtn');
        const modifiedCountBadge = document.getElementById('modifiedCountBadge');
        const dataStatusText = document.getElementById('dataStatusText');
        
        if (!saveButton || !modifiedCountBadge || !dataStatusText) return;

        const modifiedCount = this.modifiedData.size;
        
        if (modifiedCount > 0) {
            saveButton.disabled = false;
            modifiedCountBadge.textContent = `${modifiedCount} изменений`;
            modifiedCountBadge.style.display = 'inline';
            dataStatusText.textContent = `Загружено ${this.currentData.length} компаний`;
            dataStatusText.className = 'text-success';
        } else {
            saveButton.disabled = true;
            modifiedCountBadge.style.display = 'none';
            dataStatusText.textContent = `Загружено ${this.currentData.length} компаний`;
            dataStatusText.className = 'text-muted';
        }
    },

    // Показать сообщение об отсутствии данных
    showNoDataMessage() {
        const tableBody = document.getElementById('editableDataTableBody');
        const noDataMessage = document.getElementById('noDataMessage');
        const tableContainer = document.querySelector('.table-container');
        const pagination = document.getElementById('editingPagination');
        const dataStatusText = document.getElementById('dataStatusText');
        
        if (!tableBody || !noDataMessage || !tableContainer || !pagination || !dataStatusText) return;
        
        tableBody.innerHTML = '';
        noDataMessage.style.display = 'block';
        tableContainer.style.display = 'none';
        pagination.style.display = 'none';
        
        dataStatusText.textContent = 'Нет загруженных данных';
        dataStatusText.className = 'text-muted';
    },

    // Сохранение изменений
    saveChanges() {
        if (this.modifiedData.size === 0) {
            this.showToast('Нет изменений для сохранения', 'info');
            return;
        }

        // В реальном приложении здесь будет отправка на сервер
        // Для демонстрации просто обновим данные в CompanyManager
        this.modifiedData.forEach((modifiedRow, rowIndex) => {
            const company = this.currentData[rowIndex];
            
            Object.keys(modifiedRow).forEach(field => {
                const value = modifiedRow[field];
                
                // Конвертация обратно в исходные единицы измерения
                if (field === 'revenue' || field === 'net_profit' || field === 'total_taxes' || 
                    field === 'total_assets' || field === 'export_volume') {
                    company[field] = value * 1000000;
                } else {
                    company[field] = value;
                }
            });
        });

        // Обновляем данные в CompanyManager
        CompanyManager.allCompanies = [...this.currentData];
        
        // Очищаем измененные данные
        this.modifiedData.clear();
        
        // Обновляем интерфейс
        this.updateUI();
        this.renderTable();
        
        this.showToast(`Успешно сохранено ${this.modifiedData.size} изменений`, 'success');
    },

    // Экспорт данных
    exportData() {
        if (this.currentData.length === 0) {
            this.showToast('Нет данных для экспорта', 'info');
            return;
        }

        // Создаем CSV содержимое
        const headers = ['ID', 'Группа', 'Выручка (млн руб.)', 'Прибыль (млн руб.)', 'Налоги (млн руб.)', 
                       'Сотрудники', 'Активы (млн руб.)', 'Зарплата (руб.)', 'Загрузка (%)', 'Экспорт (млн руб.)'];
        
        const csvContent = [
            headers.join(','),
            ...this.currentData.map(company => [
                company.id,
                company.companyGroup,
                (company.revenue || 0) / 1000000,
                (company.net_profit || 0) / 1000000,
                (company.total_taxes || 0) / 1000000,
                company.total_staff || 0,
                (company.total_assets || 0) / 1000000,
                company.avg_salary_total || 0,
                company.production_capacity_utilization || 0,
                (company.export_volume || 0) / 1000000
            ].join(','))
        ].join('\n');

        // Создаем и скачиваем файл
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.setAttribute('href', url);
        link.setAttribute('download', `company-data-${new Date().toISOString().split('T')[0]}.csv`);
        link.style.visibility = 'hidden';
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        
        this.showToast('Данные успешно экспортированы', 'success');
    },

    // Показать уведомление
    showToast(message, type = 'success') {
        const toastMessage = document.getElementById('saveToastMessage');
        const toastElement = document.getElementById('saveToast');
        
        if (toastMessage && toastElement) {
            // Меняем стиль в зависимости от типа
            if (type === 'error') {
                toastElement.classList.remove('bg-success');
                toastElement.classList.add('bg-danger');
            } else if (type === 'info') {
                toastElement.classList.remove('bg-success');
                toastElement.classList.add('bg-info');
            } else {
                toastElement.classList.remove('bg-danger', 'bg-info');
                toastElement.classList.add('bg-success');
            }
            
            toastMessage.textContent = message;
            this.saveToast.show();
        }
    }
};

// Функция для переключения на вкладку аналитики
function switchToAnalyticsTab() {
    const analyticsTab = document.querySelector('a[data-bs-target="#analytics-tab"]');
    if (analyticsTab) {
        analyticsTab.click();
    }
}

// Функция для инициализации формы загрузки
function initUploadForm() {
    const uploadForm = document.getElementById('uploadForm');
    const companySelect = document.getElementById('company');
    
    if (uploadForm) {
        // Загружаем список компаний
        loadCompaniesForUpload();
        
        // Обработка отправки формы
        uploadForm.addEventListener('submit', function(event) {
            event.preventDefault();
            
            if (!uploadForm.checkValidity()) {
                event.stopPropagation();
                uploadForm.classList.add('was-validated');
                return;
            }
            
            // Проверяем, что файл выбран
            const fileInput = document.getElementById('file');
            if (!fileInput.files || fileInput.files.length === 0) {
                showToast('Пожалуйста, выберите файл для загрузки', 'error');
                return;
            }
            
            uploadFile();
        });
        
        // Валидация при изменении файла
        document.getElementById('file').addEventListener('change', function() {
            validateFile(this);
        });

        // Drag and drop функционал
        setupDragAndDrop();
    }
}

// Загрузка списка компаний для выбора
function loadCompaniesForUpload() {
    const companySelect = document.getElementById('company');
    
    // Здесь должен быть AJAX запрос к вашему API
    // Пример с моковыми данными:
    const companies = [
        { id: 1, name: 'ООО "Ромашка"' },
        { id: 2, name: 'АО "Луч"' },
        { id: 3, name: 'ИП Иванов' }
    ];
    
    companies.forEach(company => {
        const option = document.createElement('option');
        option.value = company.id;
        option.textContent = company.name;
        companySelect.appendChild(option);
    });
}

// Валидация файла
function validateFile(fileInput) {
    const file = fileInput.files[0];
    const maxSize = 50 * 1024 * 1024; // 50MB
    
    if (file) {
        if (file.size > maxSize) {
            alert('Файл слишком большой. Максимальный размер: 50MB');
            fileInput.value = '';
        }
    }
}

// Функция загрузки файла
function uploadFile() {
    const form = document.getElementById('uploadForm');
    const formData = new FormData(form);
    const fileInput = document.getElementById('file');
    const submitBtn = form.querySelector('button[type="submit"]');
    
    // Показываем индикатор загрузки
    const originalText = submitBtn.innerHTML;
    submitBtn.innerHTML = '<i class="bi bi-arrow-repeat spinner-border spinner-border-sm me-2"></i>Загрузка...';
    submitBtn.disabled = true;
    
    // AJAX запрос для загрузки
    fetch('/upload', {
        method: 'POST',
        body: formData
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(errorData => {
                throw new Error(errorData.error || `Ошибка сервера: ${response.status}`);
            });
        }
        return response.json();
    })
    .then(data => {
        if (data.success) {
            // Показываем уведомление об успехе
            showToast(data.message || 'Файл успешно загружен!', 'success');
            
            // Очищаем форму
            form.reset();
            form.classList.remove('was-validated');
            
            // Обновляем список документов
            if (typeof CompanyManager.loadDocuments === 'function') {
                CompanyManager.loadDocuments();
            }
            
            // Логируем дополнительную информацию
            if (data.data && data.data.inserted_count > 0) {
                console.log(`Добавлено ${data.data.inserted_count} финансовых записей`);
            }
        } else {
            throw new Error(data.error || 'Неизвестная ошибка');
        }
    })
    .catch(error => {
        console.error('Ошибка загрузки:', error);
        showToast(error.message || 'Ошибка при загрузке файла', 'error');
    })
    .finally(() => {
        // Восстанавливаем кнопку
        submitBtn.innerHTML = originalText;
        submitBtn.disabled = false;
    });
}

// Настройка drag and drop
function setupDragAndDrop() {
    const fileInput = document.getElementById('file');
    const uploadArea = document.querySelector('.file-upload-area') || 
                      document.querySelector('.card-body') || 
                      document.getElementById('upload-area-container');

    if (uploadArea) {
        ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
            uploadArea.addEventListener(eventName, preventDefaults, false);
        });

        function preventDefaults(e) {
            e.preventDefault();
            e.stopPropagation();
        }

        ['dragenter', 'dragover'].forEach(eventName => {
            uploadArea.addEventListener(eventName, highlight, false);
        });

        ['dragleave', 'drop'].forEach(eventName => {
            uploadArea.addEventListener(eventName, unhighlight, false);
        });

        function highlight() {
            uploadArea.classList.add('dragover');
        }

        function unhighlight() {
            uploadArea.classList.remove('dragover');
        }

        uploadArea.addEventListener('drop', handleDrop, false);

        function handleDrop(e) {
            const dt = e.dataTransfer;
            const files = dt.files;
            
            if (files.length > 0) {
                fileInput.files = files;
                validateFile(fileInput);
                
                // Показываем имя файла
                const fileName = files[0].name;
                const formLabel = fileInput.previousElementSibling;
                if (formLabel && formLabel.classList.contains('form-label')) {
                    formLabel.textContent = `Выбран файл: ${fileName}`;
                }
            }
        }
    }
}

// Функция показа уведомлений
function showToast(message, type = 'success') {
    const toast = document.getElementById('inviteToast');
    const toastMessage = document.getElementById('toastMessage');
    
    if (toast && toastMessage) {
        // Меняем стиль в зависимости от типа
        if (type === 'error') {
            toast.classList.remove('bg-success');
            toast.classList.add('bg-danger');
        } else {
            toast.classList.remove('bg-danger');
            toast.classList.add('bg-success');
        }
        
        toastMessage.textContent = message;
        const bsToast = new bootstrap.Toast(toast);
        bsToast.show();
    }
}

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    CompanyManager.init();
});