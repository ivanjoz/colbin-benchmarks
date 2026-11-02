# colbinbench on Lambda: x86_64

> **Superseded subsets.** These numbers predate entries 3 and 4 of the subset
> history in `datasets/README.md`: Public BI is measured on the first 16 384
> rows of each table, not the seeded sample, and the web and bulk regimes are
> pooled. Re-run before citing any of it.

colbin against MessagePack and vtprotobuf on AWS Lambda's x86_64, on the four
official datasets, run with `lambda/deploy.py invoke --arch x86_64` (see
[README.md](README.md)). The numbers compare formats with each other; they are
not the paper's matrix and are not for reporting.

**Only the `asm` build counts.** colbinbench is built normally: no
`GOEXPERIMENT`, no `purego`, so colbin runs its assembly kernels (AVX2). It is
what anyone who imports colbin gets.

## The run

| | |
|---|---|
| Date | 2026-10-27, us-east-1 |
| x86_64 | `colbin-bench-<corpus>-x86_64`, Intel Xeon @ 2.50GHz, one CPU for the whole run |
| Function | `provided.al2023`, 4096 MB, one function per corpus |
| Method | colbinbench `-worker`, one process per case, `GOMAXPROCS=1`, 5 samples of at least 100 ms each; median shown |
| colbin | 0.9.0 (`bedc044`), through `go.work` |
| colbin-benchmarks | `1a2c78f` plus uncommitted work: TPC-H's eight tables as cases, TPC-DS, the loaders and generated code |
| Go | go1.27.0 |
| Raw output | `lambda/results/20261027-105727/<corpus>-x86_64-asm.txt` for tpcds, publicbi and schemastore; `lambda/results/20261027-112342/tpch-x86_64-asm.txt` for tpch (not committed) |

The ratio columns are how many times faster colbin is: the other format's
time over colbin's, so above 1× colbin wins and below it loses. Size ratios
are the other format's bytes over colbin's, so above 1× colbin is smaller.

## Summary

Geometric mean over the cases each pair has in common.

| corpus | cases | encode vs msgpack | encode vs vtprotobuf | decode-fresh vs msgpack | decode-fresh vs vtprotobuf | decode-reuse vs msgpack | decode-reuse vs vtprotobuf | size vs msgpack | size vs vtprotobuf |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| tpch | 10 | 1.15× | 1.80× | 1.73× | 1.99× | 1.60× | 1.98× | 1.94× | 1.16× |
| tpcds | 24 | 1.01× | 1.57× | 1.56× | 1.69× | 1.05× | 1.76× | 4.22× | 1.56× |
| publicbi | 46 | 1.00× | 1.26× | 1.70× | 1.48× | 1.53× | 1.79× | 3.91× | 1.39× |
| schemastore | 54 | 0.79× | 0.88× | 0.91× | 1.07× | 1.00× | 1.31× | 1.62× | 1.10× |

## tpch

| case | size colbin | size msgpack | size vtprotobuf | encode colbin | vs msgpack | vs vtprotobuf | decode-fresh colbin | vs msgpack | vs vtprotobuf | decode-reuse colbin | vs msgpack | vs vtprotobuf |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| `tpch/sf0.1/nested` | 50,287,763 B | 162,291,941 B | 65,356,713 B | 80.12 ms | 1.18× | 1.73× | 105.61 ms | 2.44× | 2.32× | 110.07 ms | 2.13× | 2.23× |
| `tpch/sf0.1/normalized` | 65,539,427 B | 192,500,486 B | 84,468,527 B | 70.72 ms | 1.38× | 2.11× | 125.89 ms | 2.17× | 2.16× | 134.07 ms | 1.91× | 1.91× |
| `tpch/sf0.1/orders` | 12,628,985 B | 27,987,447 B | 14,678,282 B | 9.64 ms | 1.32× | 1.95× | 18.70 ms | 2.04× | 2.35× | 19.05 ms | 1.88× | 2.16× |
| `tpch/sf0.1/lineitem` | 38,068,699 B | 142,797,489 B | 53,517,301 B | 54.96 ms | 1.38× | 2.17× | 102.87 ms | 2.05× | 1.85× | 103.15 ms | 1.93× | 1.84× |
| `tpch/sf0.1/customer` | 2,227,935 B | 3,301,879 B | 2,426,154 B | 894.61 µs | 1.35× | 1.98× | 3.82 ms | 1.55× | 1.74× | 3.27 ms | 1.82× | 1.95× |
| `tpch/sf0.1/part` | 2,091,567 B | 3,539,513 B | 2,392,716 B | 1.30 ms | 1.38× | 2.05× | 4.81 ms | 1.96× | 2.35× | 4.79 ms | 1.33× | 2.18× |
| `tpch/sf0.1/partsupp` | 10,391,904 B | 14,682,502 B | 11,311,721 B | 2.68 ms | 1.56× | 2.32× | 8.47 ms | 1.34× | 1.78× | 7.00 ms | 1.55× | 2.11× |
| `tpch/sf0.1/supplier` | 127,818 B | 188,112 B | 139,677 B | 46.60 µs | 1.34× | 2.15× | 205.76 µs | 1.85× | 2.16× | 218.48 µs | 1.62× | 2.02× |
| `tpch/sf0.1/nation` | 2,125 B | 3,017 B | 2,274 B | 1.07 µs | 0.68× | 1.27× | 3.63 µs | 1.42× | 1.89× | 3.78 µs | 1.23× | 1.77× |
| `tpch/sf0.1/region` | 394 B | 509 B | 402 B | 302 ns | 0.54× | 0.90× | 977 ns | 1.03× | 1.50× | 801 ns | 1.02× | 1.73× |

## tpcds

| case | size colbin | size msgpack | size vtprotobuf | encode colbin | vs msgpack | vs vtprotobuf | decode-fresh colbin | vs msgpack | vs vtprotobuf | decode-reuse colbin | vs msgpack | vs vtprotobuf |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| `tpcds/sf0.1/call_center` | 295 B | 564 B | 302 B | 284 ns | 0.94× | 1.40× | 1.20 µs | 1.27× | 1.15× | 1.05 µs | 1.16× | 1.25× |
| `tpcds/sf0.1/catalog_page` | 1,351,800 B | 2,791,936 B | 1,579,274 B | 805.30 µs | 1.09× | 1.84× | 5.47 ms | 1.61× | 1.83× | 4.63 ms | 0.96× | 1.98× |
| `tpcds/sf0.1/catalog_returns` | 670,263 B | 6,483,309 B | 1,394,385 B | 2.37 ms | 1.19× | 1.66× | 9.16 ms | 1.41× | 1.54× | 5.06 ms | 1.28× | 2.61× |
| `tpcds/sf0.1/catalog_sales` | 9,029,096 B | 75,292,197 B | 19,124,475 B | 39.42 ms | 0.92× | 1.34× | 51.96 ms | 2.97× | 2.74× | 55.78 ms | 1.43× | 2.48× |
| `tpcds/sf0.1/customer` | 869,708 B | 3,388,889 B | 1,183,302 B | 1.43 ms | 1.16× | 1.63× | 7.04 ms | 1.54× | 1.66× | 8.50 ms | 0.56× | 1.35× |
| `tpcds/sf0.1/customer_address` | 514,185 B | 1,136,274 B | 593,793 B | 775.89 µs | 1.07× | 1.51× | 5.44 ms | 1.17× | 1.16× | 4.73 ms | 0.65× | 1.29× |
| `tpcds/sf0.1/customer_demographics` | 4,639,212 B | 28,852,063 B | 7,446,050 B | 8.82 ms | 1.53× | 2.34× | 21.18 ms | 1.76× | 1.90× | 22.33 ms | 1.53× | 1.65× |
| `tpcds/sf0.1/date_dim` | 3,959,873 B | 25,914,256 B | 9,644,995 B | 11.57 ms | 1.37× | 2.08× | 25.80 ms | 1.49× | 1.31× | 25.04 ms | 1.39× | 1.38× |
| `tpcds/sf0.1/household_demographics` | 70,090 B | 507,355 B | 143,633 B | 111.69 µs | 1.67× | 3.46× | 526.98 µs | 1.81× | 2.84× | 464.31 µs | 1.52× | 3.19× |
| `tpcds/sf0.1/income_band` | 116 B | 933 B | 234 B | 747 ns | 0.47× | 0.96× | 1.35 µs | 0.92× | 2.09× | 934 ns | 0.99× | 2.94× |
| `tpcds/sf0.1/inventory` | 458,503 B | 12,570,391 B | 3,442,896 B | 2.75 ms | 2.24× | 4.52× | 7.73 ms | 2.50× | 3.71× | 7.93 ms | 1.77× | 3.25× |
| `tpcds/sf0.1/item` | 431,607 B | 820,367 B | 496,246 B | 393.23 µs | 1.05× | 1.54× | 2.42 ms | 1.54× | 1.38× | 2.43 ms | 0.70× | 1.38× |
| `tpcds/sf0.1/promotion` | 2,940 B | 9,803 B | 3,819 B | 6.69 µs | 0.85× | 1.23× | 25.70 µs | 1.52× | 1.51× | 27.67 µs | 0.62× | 1.40× |
| `tpcds/sf0.1/reason` | 315 B | 504 B | 321 B | 366 ns | 0.56× | 0.99× | 1.14 µs | 1.14× | 1.65× | 1.05 µs | 1.01× | 1.72× |
| `tpcds/sf0.1/ship_mode` | 1,075 B | 2,071 B | 1,222 B | 1.44 µs | 0.76× | 1.21× | 4.32 µs | 1.54× | 1.84× | 4.34 µs | 1.35× | 1.80× |
| `tpcds/sf0.1/store` | 272 B | 550 B | 279 B | 304 ns | 0.93× | 1.27× | 1.53 µs | 0.97× | 0.90× | 1.36 µs | 0.84× | 0.95× |
| `tpcds/sf0.1/store_returns` | 1,023,631 B | 8,098,951 B | 1,990,265 B | 3.53 ms | 1.20× | 1.74× | 10.43 ms | 1.84× | 1.83× | 6.70 ms | 1.38× | 2.57× |
| `tpcds/sf0.1/store_sales` | 12,291,947 B | 92,053,716 B | 23,213,533 B | 49.38 ms | 1.03× | 1.51× | 73.24 ms | 2.93× | 2.70× | 78.59 ms | 1.38× | 2.49× |
| `tpcds/sf0.1/time_dim` | 3,383,881 B | 10,160,827 B | 4,961,612 B | 4.84 ms | 1.28× | 2.21× | 12.26 ms | 1.86× | 1.94× | 11.67 ms | 1.83× | 1.87× |
| `tpcds/sf0.1/warehouse` | 128 B | 268 B | 135 B | 195 ns | 0.87× | 1.20× | 855 ns | 1.03× | 0.98× | 645 ns | 1.05× | 1.23× |
| `tpcds/sf0.1/web_page` | 431 B | 1,303 B | 475 B | 1.39 µs | 0.41× | 0.66× | 5.06 µs | 0.78× | 0.80× | 4.98 µs | 0.44× | 0.79× |
| `tpcds/sf0.1/web_returns` | 309,143 B | 2,840,038 B | 596,275 B | 943.58 µs | 1.28× | 1.86× | 3.90 ms | 2.42× | 2.39× | 3.75 ms | 0.80× | 2.29× |
| `tpcds/sf0.1/web_sales` | 4,492,649 B | 36,919,650 B | 9,501,012 B | 20.03 ms | 0.92× | 1.34× | 25.50 ms | 3.01× | 2.96× | 27.87 ms | 1.42× | 2.50× |
| `tpcds/sf0.1/web_site` | 936 B | 1,571 B | 932 B | 674 ns | 0.96× | 1.42× | 3.61 µs | 1.07× | 1.05× | 3.49 µs | 0.93× | 1.06× |

## publicbi

| case | size colbin | size msgpack | size vtprotobuf | encode colbin | vs msgpack | vs vtprotobuf | decode-fresh colbin | vs msgpack | vs vtprotobuf | decode-reuse colbin | vs msgpack | vs vtprotobuf |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| `publicbi/Arade` | 429,090 B | 1,547,809 B | 760,032 B | 717.59 µs | 1.25× | 2.24× | 2.10 ms | 2.02× | 2.25× | 1.52 ms | 2.10× | 3.00× |
| `publicbi/Bimbo` | 152,706 B | 2,803,533 B | 565,006 B | 503.38 µs | 1.57× | 2.80× | 1.38 ms | 2.12× | 2.23× | 999.39 µs | 2.53× | 2.86× |
| `publicbi/CMSprovider` | 2,910,186 B | 11,441,545 B | 3,981,197 B | 5.39 ms | 0.88× | 1.10× | 17.18 ms | 1.22× | 1.14× | 12.24 ms | 0.99× | 1.51× |
| `publicbi/CityMaxCapita` | 4,871,767 B | 10,324,995 B | 5,673,979 B | 4.57 ms | 1.10× | 1.40× | 14.40 ms | 1.33× | 1.20× | 9.69 ms | 1.50× | 1.80× |
| `publicbi/CommonGovernment` | 12,355,203 B | 26,194,863 B | 13,999,940 B | 12.01 ms | 0.91× | 1.20× | 19.32 ms | 1.88× | 1.61× | 18.68 ms | 1.63× | 1.57× |
| `publicbi/Corporations` | 935,086 B | 6,418,297 B | 1,185,984 B | 1.86 ms | 1.16× | 1.13× | 5.32 ms | 2.15× | 2.19× | 5.05 ms | 1.54× | 2.30× |
| `publicbi/Eixo` | 11,533,629 B | 33,900,039 B | 13,885,072 B | 19.87 ms | 0.82× | 1.04× | 27.47 ms | 2.19× | 1.82× | 27.09 ms | 1.65× | 1.78× |
| `publicbi/Euro2016` | 2,467,764 B | 4,789,556 B | 2,826,864 B | 1.33 ms | 1.10× | 1.67× | 5.26 ms | 1.83× | 2.19× | 6.32 ms | 0.81× | 1.83× |
| `publicbi/Food` | 371,255 B | 1,752,413 B | 595,017 B | 525.42 µs | 1.35× | 2.19× | 2.67 ms | 1.45× | 2.13× | 2.18 ms | 1.13× | 2.57× |
| `publicbi/Generico` | 8,391,244 B | 15,073,854 B | 9,753,412 B | 8.05 ms | 0.89× | 1.20× | 16.44 ms | 1.72× | 1.52× | 11.99 ms | 1.77× | 2.04× |
| `publicbi/HashTags` | 17,740,354 B | 59,233,011 B | 19,383,412 B | 16.93 ms | 1.15× | 1.01× | 24.48 ms | 2.60× | 1.42× | 23.95 ms | 2.36× | 1.44× |
| `publicbi/Hatred` | 4,782,872 B | 9,759,342 B | 5,723,926 B | 4.28 ms | 1.17× | 1.53× | 12.72 ms | 1.32× | 1.21× | 7.96 ms | 1.79× | 2.09× |
| `publicbi/IGlocations1` | 791,857 B | 4,467,040 B | 1,145,311 B | 1.23 ms | 1.34× | 2.08× | 3.63 ms | 2.19× | 1.76× | 2.63 ms | 2.19× | 2.35× |
| `publicbi/IGlocations2` | 6,107,689 B | 10,991,946 B | 6,695,495 B | 2.83 ms | 1.30× | 1.73× | 9.11 ms | 1.36× | 1.51× | 6.40 ms | 1.78× | 1.87× |
| `publicbi/IUBLibrary` | 342,672 B | 955,371 B | 424,679 B | 379.20 µs | 1.18× | 1.70× | 1.81 ms | 1.39× | 1.45× | 1.74 ms | 0.97× | 1.48× |
| `publicbi/MLB` | 1,443,222 B | 6,419,595 B | 2,553,727 B | 3.16 ms | 1.40× | 2.18× | 9.27 ms | 2.05× | 1.61× | 6.98 ms | 2.17× | 2.38× |
| `publicbi/MedPayment1` | 2,826,322 B | 12,097,960 B | 3,826,463 B | 5.31 ms | 0.92× | 1.08× | 13.90 ms | 1.48× | 1.35× | 10.94 ms | 1.16× | 1.61× |
| `publicbi/MedPayment2` | 2,925,674 B | 12,716,577 B | 4,002,317 B | 5.37 ms | 0.91× | 1.02× | 14.26 ms | 1.48× | 1.35× | 11.09 ms | 1.17× | 1.64× |
| `publicbi/Medicare1` | 2,012,962 B | 10,433,638 B | 2,904,214 B | 2.89 ms | 1.20× | 1.49× | 8.91 ms | 1.57× | 1.44× | 5.43 ms | 2.00× | 2.76× |
| `publicbi/Medicare2` | 2,750,289 B | 11,944,222 B | 3,656,663 B | 5.05 ms | 0.93× | 1.07× | 13.69 ms | 1.46× | 1.35× | 9.27 ms | 1.33× | 1.84× |
| `publicbi/Medicare3` | 3,201,368 B | 12,793,384 B | 4,343,774 B | 6.50 ms | 0.92× | 1.04× | 13.84 ms | 1.73× | 1.61× | 10.68 ms | 1.32× | 1.94× |
| `publicbi/Motos` | 8,398,875 B | 15,353,438 B | 9,755,349 B | 8.08 ms | 0.91× | 1.22× | 14.23 ms | 2.00× | 1.85× | 12.57 ms | 1.72× | 1.93× |
| `publicbi/MulheresMil` | 12,178,510 B | 35,051,414 B | 14,565,924 B | 20.97 ms | 0.84× | 1.03× | 27.36 ms | 2.28× | 1.85× | 28.27 ms | 1.65× | 1.87× |
| `publicbi/NYC` | 6,626,692 B | 19,906,854 B | 7,639,506 B | 9.18 ms | 0.89× | 1.07× | 17.31 ms | 1.73× | 1.35× | 13.98 ms | 1.72× | 1.59× |
| `publicbi/PanCreactomy1` | 3,203,868 B | 12,794,485 B | 4,343,794 B | 6.53 ms | 0.88× | 1.06× | 13.59 ms | 1.75× | 1.63× | 10.57 ms | 1.34× | 1.96× |
| `publicbi/PanCreactomy2` | 3,194,066 B | 12,783,651 B | 4,332,030 B | 6.45 ms | 0.92× | 1.08× | 14.14 ms | 1.67× | 1.56× | 10.32 ms | 1.37× | 2.00× |
| `publicbi/Physicians` | 2,829,659 B | 12,097,960 B | 3,826,463 B | 5.42 ms | 0.91× | 1.00× | 13.86 ms | 1.49× | 1.37× | 10.84 ms | 1.17× | 1.63× |
| `publicbi/Provider` | 2,757,107 B | 11,956,015 B | 3,667,589 B | 5.02 ms | 0.94× | 1.07× | 12.74 ms | 1.57× | 1.42× | 9.34 ms | 1.30× | 1.84× |
| `publicbi/RealEstate1` | 3,091,890 B | 10,198,092 B | 4,142,593 B | 3.53 ms | 1.25× | 1.67× | 13.99 ms | 1.22× | 1.15× | 7.91 ms | 1.82× | 1.85× |
| `publicbi/RealEstate2` | 3,773,838 B | 10,484,133 B | 4,467,329 B | 4.82 ms | 1.10× | 1.40× | 15.89 ms | 1.24× | 1.15× | 12.79 ms | 1.17× | 1.37× |
| `publicbi/Redfin1` | 2,591,220 B | 13,277,894 B | 3,668,795 B | 5.22 ms | 0.97× | 0.86× | 10.76 ms | 1.81× | 1.50× | 7.76 ms | 1.69× | 1.85× |
| `publicbi/Redfin2` | 2,563,695 B | 13,258,418 B | 3,644,491 B | 6.09 ms | 0.87× | 0.77× | 11.05 ms | 1.79× | 1.43× | 8.10 ms | 1.64× | 1.80× |
| `publicbi/Redfin3` | 2,607,025 B | 14,247,764 B | 3,818,983 B | 6.59 ms | 0.85× | 0.76× | 12.64 ms | 1.65× | 1.34× | 8.14 ms | 1.74× | 1.85× |
| `publicbi/Redfin4` | 2,809,165 B | 14,951,333 B | 4,062,573 B | 7.43 ms | 0.81× | 0.80× | 10.82 ms | 2.01× | 1.60× | 9.56 ms | 1.54× | 1.73× |
| `publicbi/Rentabilidad` | 8,729,704 B | 60,474,564 B | 12,372,349 B | 22.22 ms | 1.02× | 1.44× | 29.06 ms | 2.33× | 1.44× | 28.44 ms | 2.03× | 1.35× |
| `publicbi/Romance` | 4,993,460 B | 6,566,110 B | 5,390,537 B | 2.67 ms | 0.97× | 1.45× | 10.34 ms | 1.34× | 1.53× | 9.37 ms | 0.81× | 1.56× |
| `publicbi/SalariesFrance` | 6,505,971 B | 18,809,828 B | 7,520,987 B | 9.31 ms | 0.85× | 0.89× | 14.09 ms | 1.95× | 1.46× | 13.32 ms | 1.62× | 1.53× |
| `publicbi/TableroSistemaPenal` | 4,028,567 B | 9,371,541 B | 4,800,824 B | 3.59 ms | 1.23× | 1.73× | 14.67 ms | 1.20× | 1.16× | 9.00 ms | 1.68× | 1.64× |
| `publicbi/Taxpayer` | 2,767,682 B | 11,967,005 B | 3,681,302 B | 5.32 ms | 0.91× | 1.05× | 13.27 ms | 1.51× | 1.39× | 9.29 ms | 1.33× | 1.84× |
| `publicbi/Telco` | 5,400,547 B | 56,119,053 B | 12,165,854 B | 15.75 ms | 1.24× | 2.09× | 27.96 ms | 2.21× | 1.00× | 22.47 ms | 2.54× | 1.18× |
| `publicbi/TrainsUK1` | 7 B | 39 B | 10 B | 135 ns | 0.41× | 0.61× | 311 ns | 0.64× | 0.77× | 168 ns | 0.57× | 1.24× |
| `publicbi/TrainsUK2` | 3,339,055 B | 16,059,596 B | 4,837,231 B | 6.84 ms | 1.03× | 1.32× | 13.50 ms | 2.14× | 1.88× | 9.24 ms | 2.05× | 2.64× |
| `publicbi/USCensus` | 2,676,544 B | 64,535,299 B | 9,010,098 B | 52.36 ms | 1.16× | 1.00× | 52.21 ms | 3.92× | 1.02× | 55.00 ms | 3.52× | 0.98× |
| `publicbi/Uberlandia` | 12,046,190 B | 35,000,897 B | 14,474,997 B | 21.04 ms | 0.80× | 1.01× | 28.26 ms | 2.10× | 1.77× | 27.87 ms | 1.62× | 1.75× |
| `publicbi/Wins` | 6,460,522 B | 37,157,025 B | 14,193,162 B | 45.01 ms | 0.86× | 1.36× | 52.22 ms | 2.38× | 1.80× | 54.13 ms | 1.76× | 1.59× |
| `publicbi/YaleLanguages` | 3,178,642 B | 9,594,151 B | 3,999,364 B | 5.88 ms | 0.89× | 1.34× | 24.09 ms | 0.94× | 0.90× | 15.44 ms | 0.98× | 1.34× |

## schemastore

colbin has no result for 6 of the 54 cases (`schemastore/geojson`, `schemastore/gruntcontribclean`, `schemastore/nightwatch`, `schemastore/openweatherroadrisk`, `schemastore/packagejsonlintrc`, `schemastore/tslintmulti`); vtprotobuf has none for the `dynamic` cases. They show as n/a.

| case | size colbin | size msgpack | size vtprotobuf | encode colbin | vs msgpack | vs vtprotobuf | decode-fresh colbin | vs msgpack | vs vtprotobuf | decode-reuse colbin | vs msgpack | vs vtprotobuf |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| `schemastore/circleciblank` | 2 B | 14 B | 5 B | 69 ns | 0.72× | 0.68× | 82 ns | 0.99× | 1.38× | 55 ns | 1.04× | 1.12× |
| `schemastore/circlecimatrix` | 26 B | 72 B | 26 B | 163 ns | 0.41× | 0.88× | 731 ns | 0.44× | 0.89× | 485 ns | 0.33× | 1.23× |
| `schemastore/commitlint` | 58 B | 220 B | 62 B | 442 ns | 0.43× | 0.53× | 1.73 µs | 0.65× | 0.95× | 1.42 µs | 0.36× | 1.11× |
| `schemastore/commitlintbasic` | 0 B | 17 B | 0 B | 51 ns | 0.89× | 0.87× | 65 ns | 1.07× | 1.71× | 49 ns | 1.07× | 1.26× |
| `schemastore/epr` | 238 B | 444 B | 247 B | 407 ns | 0.68× | 0.82× | 1.79 µs | 0.89× | 1.11× | 1.24 µs | 0.84× | 1.52× |
| `schemastore/eslintrc` | 138 B | 908 B | 161 B | 477 ns | 0.50× | 0.85× | 1.53 µs | 0.88× | 0.86× | 1.22 µs | 0.84× | 1.00× |
| `schemastore/esmrc` | 20 B | 64 B | 23 B | 98 ns | 0.76× | 1.01× | 406 ns | 0.85× | 0.94× | 223 ns | 0.94× | 1.42× |
| `schemastore/geojson` | n/a | 564 B | 325 B | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| `schemastore/githubfundingblank` | 17 B | 121 B | 17 B | 94 ns | 0.73× | 0.64× | 257 ns | 1.15× | 0.76× | 155 ns | 1.27× | 0.83× |
| `schemastore/githubworkflow` | 183 B | 388 B | 189 B | 421 ns | 0.60× | 0.74× | 1.45 µs | 0.94× | 1.22× | 1.13 µs | 0.87× | 1.52× |
| `schemastore/gruntcontribclean` | n/a | 59 B | 22 B | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| `schemastore/imageoptimizerwebjob` | 22 B | 61 B | 23 B | 173 ns | 0.45× | 0.57× | 646 ns | 0.63× | 0.73× | 307 ns | 0.58× | 1.41× |
| `schemastore/jsonereversesort` | 19 B | 46 B | 21 B | 139 ns | 0.56× | 0.96× | 555 ns | 0.52× | 0.90× | 406 ns | 0.39× | 1.14× |
| `schemastore/jsonesort` | 9 B | 18 B | 10 B | 72 ns | 0.88× | 1.14× | 236 ns | 0.94× | 0.89× | 98 ns | 1.00× | 1.65× |
| `schemastore/jsonfeed` | 412 B | 511 B | 413 B | 230 ns | 0.74× | 1.02× | 950 ns | 1.13× | 1.21× | 762 ns | 1.17× | 1.44× |
| `schemastore/jsonresume` | 2,208 B | 2,749 B | 2,225 B | 1.19 µs | 0.74× | 1.03× | 7.66 µs | 0.86× | 1.02× | 4.96 µs | 0.99× | 1.52× |
| `schemastore/netcoreproject` | 278 B | 880 B | 284 B | 328 ns | 0.99× | 1.32× | 1.35 µs | 1.31× | 1.46× | 1.09 µs | 1.39× | 1.73× |
| `schemastore/nightwatch` | n/a | 1,111 B | 120 B | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| `schemastore/openweathermap` | 171 B | 379 B | 188 B | 364 ns | 0.55× | 0.87× | 1.24 µs | 0.75× | 0.93× | 978 ns | 0.70× | 1.10× |
| `schemastore/openweatherroadrisk` | n/a | 369 B | 173 B | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| `schemastore/packagejson` | 1,569 B | 1,978 B | 1,581 B | 777 ns | 0.86× | 1.33× | 4.26 µs | 0.98× | 1.21× | 3.15 µs | 1.14× | 1.58× |
| `schemastore/packagejsonlintrc` | n/a | 1,392 B | 454 B | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| `schemastore/sapcloudsdkpipeline` | 0 B | 25 B | 0 B | 58 ns | 0.80× | 0.79× | 124 ns | 1.08× | 0.93× | 71 ns | 1.01× | 0.81× |
| `schemastore/travisnotifications` | 521 B | 627 B | 521 B | 217 ns | 0.60× | 1.02× | 905 ns | 0.89× | 1.33× | 802 ns | 0.89× | 1.44× |
| `schemastore/tslintbasic` | 7 B | 49 B | 8 B | 78 ns | 0.61× | 0.95× | 197 ns | 0.53× | 1.57× | 176 ns | 0.49× | 1.59× |
| `schemastore/tslintextend` | 47 B | 58 B | 47 B | 82 ns | 0.80× | 0.98× | 348 ns | 0.79× | 1.02× | 184 ns | 0.92× | 1.74× |
| `schemastore/tslintmulti` | n/a | 65 B | 14 B | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| `schemastore/circleciblank/dynamic` | 10 B | 18 B | n/a | 131 ns | 0.89× | n/a | 518 ns | 0.74× | n/a | 298 ns | 1.27× | n/a |
| `schemastore/circlecimatrix/dynamic` | 72 B | 96 B | n/a | 925 ns | 0.86× | n/a | 3.00 µs | 0.95× | n/a | 2.75 µs | 1.01× | n/a |
| `schemastore/commitlint/dynamic` | 74 B | 90 B | n/a | 347 ns | 0.87× | n/a | 1.72 µs | 1.02× | n/a | 1.47 µs | 1.19× | n/a |
| `schemastore/commitlintbasic/dynamic` | 17 B | 17 B | n/a | 127 ns | 0.91× | n/a | 481 ns | 0.78× | n/a | 208 ns | 1.77× | n/a |
| `schemastore/epr/dynamic` | 412 B | 416 B | n/a | 1.07 µs | 0.90× | n/a | 4.77 µs | 1.14× | n/a | 4.47 µs | 1.22× | n/a |
| `schemastore/eslintrc/dynamic` | 970 B | 1,283 B | n/a | 1.72 µs | 0.95× | n/a | 7.33 µs | 1.18× | n/a | 7.09 µs | 1.21× | n/a |
| `schemastore/esmrc/dynamic` | 64 B | 64 B | n/a | 251 ns | 0.94× | n/a | 1.23 µs | 0.94× | n/a | 1.02 µs | 1.13× | n/a |
| `schemastore/geojson/dynamic` | 172 B | 322 B | n/a | 804 ns | 0.61× | n/a | 4.20 µs | 1.08× | n/a | 3.91 µs | 1.15× | n/a |
| `schemastore/githubfundingblank/dynamic` | 124 B | 124 B | n/a | 348 ns | 0.94× | n/a | 2.05 µs | 0.72× | n/a | 1.06 µs | 1.37× | n/a |
| `schemastore/githubworkflow/dynamic` | 287 B | 287 B | n/a | 1.18 µs | 0.95× | n/a | 4.71 µs | 1.08× | n/a | 4.47 µs | 1.12× | n/a |
| `schemastore/gruntcontribclean/dynamic` | 60 B | 60 B | n/a | 438 ns | 0.92× | n/a | 1.94 µs | 0.96× | n/a | 1.73 µs | 1.06× | n/a |
| `schemastore/imageoptimizerwebjob/dynamic` | 61 B | 61 B | n/a | 310 ns | 0.89× | n/a | 1.50 µs | 0.94× | n/a | 1.28 µs | 1.09× | n/a |
| `schemastore/jsonereversesort/dynamic` | 53 B | 92 B | n/a | 644 ns | 0.89× | n/a | 2.46 µs | 0.96× | n/a | 2.22 µs | 1.02× | n/a |
| `schemastore/jsonesort/dynamic` | 21 B | 61 B | n/a | 214 ns | 0.87× | n/a | 1.08 µs | 0.83× | n/a | 833 ns | 1.06× | n/a |
| `schemastore/jsonfeed/dynamic` | 517 B | 517 B | n/a | 573 ns | 1.01× | n/a | 2.99 µs | 1.14× | n/a | 2.69 µs | 1.22× | n/a |
| `schemastore/jsonresume/dynamic` | 2,749 B | 2,749 B | n/a | 3.22 µs | 0.99× | n/a | 17.23 µs | 1.19× | n/a | 16.49 µs | 1.25× | n/a |
| `schemastore/netcoreproject/dynamic` | 918 B | 919 B | n/a | 1.56 µs | 0.96× | n/a | 6.94 µs | 1.20× | n/a | 6.67 µs | 1.25× | n/a |
| `schemastore/nightwatch/dynamic` | 1,179 B | 1,264 B | n/a | 2.16 µs | 0.97× | n/a | 14.19 µs | 0.78× | n/a | 9.18 µs | 1.19× | n/a |
| `schemastore/openweathermap/dynamic` | 387 B | 465 B | n/a | 1.34 µs | 0.93× | n/a | 6.04 µs | 1.03× | n/a | 5.18 µs | 1.17× | n/a |
| `schemastore/openweatherroadrisk/dynamic` | 343 B | 369 B | n/a | 955 ns | 0.96× | n/a | 4.02 µs | 1.10× | n/a | 3.92 µs | 1.10× | n/a |
| `schemastore/packagejson/dynamic` | 1,991 B | 1,995 B | n/a | 2.24 µs | 1.00× | n/a | 13.14 µs | 1.08× | n/a | 10.96 µs | 1.28× | n/a |
| `schemastore/packagejsonlintrc/dynamic` | 988 B | 989 B | n/a | 1.50 µs | 0.94× | n/a | 8.23 µs | 1.26× | n/a | 7.88 µs | 1.30× | n/a |
| `schemastore/sapcloudsdkpipeline/dynamic` | 25 B | 25 B | n/a | 160 ns | 0.91× | n/a | 579 ns | 0.91× | n/a | 302 ns | 1.73× | n/a |
| `schemastore/travisnotifications/dynamic` | 627 B | 627 B | n/a | 949 ns | 1.00× | n/a | 4.00 µs | 1.04× | n/a | 3.71 µs | 1.11× | n/a |
| `schemastore/tslintbasic/dynamic` | 51 B | 51 B | n/a | 444 ns | 0.89× | n/a | 1.46 µs | 0.92× | n/a | 1.26 µs | 1.06× | n/a |
| `schemastore/tslintextend/dynamic` | 55 B | 55 B | n/a | 158 ns | 0.89× | n/a | 918 ns | 0.78× | n/a | 673 ns | 1.08× | n/a |
| `schemastore/tslintmulti/dynamic` | 68 B | 68 B | n/a | 508 ns | 0.86× | n/a | 1.93 µs | 0.94× | n/a | 1.71 µs | 1.05× | n/a |

