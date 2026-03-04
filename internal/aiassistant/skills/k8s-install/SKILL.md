---
name: k8s-install
description: K8s 集群安装指导。通过对话询问用户安装方式、节点信息，分步骤输出命令，每步等待用户确认。不嵌入安装包，优先使用阿里云镜像下载，国内网络更稳定。
---

# K8s 集群安装知识库

> **优先级说明**：本技能为通用知识库，先于数据库 system_prompt 注入。数据库中的定制（如安装路径、版本偏好）会追加在后，**优先于本技能默认值**。改数据库即可生效，无需改 SKILL。

本技能用于指导用户安装 Kubernetes 集群，**不嵌入任何二进制**，优先使用**阿里云镜像**下载，国内网络更稳定。安装涉及 containerd、CNI、kubeadm/二进制等，需分步骤进行，每步后等待用户确认。

## 一、版本占位符（必须遵守）

**所有 URL 和命令中的版本号必须使用占位符，并根据「用户指定」或「默认策略」动态替换**：

| 占位符 | 含义 | 示例值 | 说明 |
|--------|------|--------|------|
| `{K8S_VERSION}` | K8s 版本 | v1.29、v1.30、v1.31 | 用户指定或询问后确认，格式 vX.Y 或 vX.Y.Z |
| `{CONTAINERD_VERSION}` | containerd 版本 | 1.7.18、1.7.30 | 用户指定或推荐与 K8s 兼容的稳定版 |
| `{CALICO_VERSION}` | Calico 版本 | v3.28、v3.29 | 用户指定或推荐与 K8s 兼容的版本 |
| `{RUNC_VERSION}` | runc 版本 | 1.1.12 | containerd 依赖 |
| `{CNI_PLUGINS_VERSION}` | CNI 插件版本 | 1.4.0 | 如 v1.4.0 |
| `{ETCD_VERSION}` | etcd 版本 | v3.5.16、v3.5.17 | 与 K8s 版本兼容，推荐 3.5.x |

Calico、containerd、etcd 等版本需与 K8s 版本兼容，用户未指定时按 K8s 版本选取对应兼容的稳定版。

**规则**：
- **用户未指定版本时**：使用当前最新稳定版**落后 1～2 个小版本**（如最新为 v1.35 则用 v1.33 或 v1.34），默认无需追问；若用户明确要求“指定版本”再改为其指定值
- 输出命令时**必须将占位符替换为实际版本号**，禁止在给用户的命令中保留 `{xxx}` 未替换
- 阿里云 kubernetes-new 支持 v1.24～v1.35，超出范围需提示用户

## 二、阿里云镜像地址（URL 格式，版本用占位符）

### 1. Kubernetes（kubeadm、kubelet、kubectl）

**方式一：yum/apt 安装（推荐）**：
- RPM：`https://mirrors.aliyun.com/kubernetes-new/core/stable/{K8S_VERSION}/rpm/`
- DEB：`https://mirrors.aliyun.com/kubernetes-new/core/stable/{K8S_VERSION}/deb/`
- gpgkey：`https://mirrors.aliyun.com/kubernetes-new/core/stable/{K8S_VERSION}/rpm/repodata/repomd.xml.key`

**方式二：二进制**：`https://dl.k8s.io/{K8S_VERSION}/bin/linux/amd64/{kubectl|kubelet|kubeadm}`

### 2. containerd

**方式一：阿里云 docker-ce 源**：`yum/apt install containerd.io`（repo 无版本占位符）

**方式二：二进制**：`https://ghproxy.com/https://github.com/containerd/containerd/releases/download/v{CONTAINERD_VERSION}/containerd-{CONTAINERD_VERSION}-linux-amd64.tar.gz`

### 3. runc

- `https://ghproxy.com/https://github.com/opencontainers/runc/releases/download/v{RUNC_VERSION}/runc.amd64`

### 4. CNI 插件

**Calico**：
- calico.yaml：`https://ghproxy.com/https://raw.githubusercontent.com/projectcalico/calico/{CALICO_VERSION}/manifests/calico.yaml`
- canal.yaml：`https://ghproxy.com/https://raw.githubusercontent.com/projectcalico/calico/{CALICO_VERSION}/manifests/canal.yaml`

**Flannel**：`https://ghproxy.com/https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml`（无版本占位符，用 latest）

**CNI 二进制**：`https://ghproxy.com/https://github.com/containernetworking/plugins/releases/download/v{CNI_PLUGINS_VERSION}/cni-plugins-linux-amd64-v{CNI_PLUGINS_VERSION}.tgz`

### 5. 容器镜像加速（k8s 组件镜像）

- 阿里云镜像：`registry.aliyuncs.com/google_containers`
- kubeadm init 时：`--image-repository=registry.aliyuncs.com/google_containers`

### 6. cri-tools

- 阿里云 kubernetes-new 源已包含，yum/apt 安装 kubeadm 时一并安装

### 7. etcd（独立部署时）

- 二进制：`https://ghproxy.com/https://github.com/etcd-io/etcd/releases/download/{ETCD_VERSION}/etcd-{ETCD_VERSION}-linux-amd64.tar.gz`
- 官方：`https://github.com/etcd-io/etcd/releases/download/{ETCD_VERSION}/etcd-{ETCD_VERSION}-linux-amd64.tar.gz`

**etcd 部署模式**：
- **1 个节点**：单机模式，直接启动 etcd 服务
- **≥3 个节点**：集群模式，需配置 `initial-cluster`、`initial-cluster-token`，奇数节点（3/5/7）保证 quorum

## 三、网络环境与源策略（必须先判断）

在输出安装命令前，先判断目标机器网络条件，并选择对应策略：

1. **公网可达**（可直接访问 mirrors.aliyun.com / github / registry）  
   - 使用本技能默认阿里云与官方加速地址

2. **内网受限**（目标机不能直连公网，但可访问公司内网源）  
   - 使用企业内网源/制品库/镜像仓库，禁止给出公网 URL
   - 使用以下占位符（由用户提供实际地址）：
     - `{INTERNAL_YUM_REPO}`
     - `{INTERNAL_APT_REPO}`
     - `{INTERNAL_BINARY_REPO}`（二进制包下载地址）
     - `{INTERNAL_IMAGE_REGISTRY}`（容器镜像仓库）

3. **完全离线**（目标机既不能公网也不能内网源）  
   - 走“中转机打包 -> 离线拷贝 -> 本地安装”流程
   - 包括：rpm/deb 包、k8s/containerd/etcd 二进制、CNI YAML、容器镜像离线包

### 3.1 内网受限场景规则

- `kubeadm init` 使用内网镜像仓库：`--image-repository={INTERNAL_IMAGE_REGISTRY}`
- 包管理安装改为企业源（yum/apt 的 baseurl 改内网地址）
- 二进制下载改为 `{INTERNAL_BINARY_REPO}`，例如：
  - `{INTERNAL_BINARY_REPO}/kubernetes/{K8S_VERSION}/kubeadm`
  - `{INTERNAL_BINARY_REPO}/containerd/{CONTAINERD_VERSION}/containerd-{CONTAINERD_VERSION}-linux-amd64.tar.gz`

### 3.2 完全离线场景规则

- 在可联网中转机准备安装物料并校验 checksum
- 通过 `scp/rsync/u盘/NAS` 分发到目标节点
- 镜像离线导入（示例）：
  - `ctr -n k8s.io images import k8s-images.tar`
  - 或 `docker load -i k8s-images.tar` 后推送到内网私有仓库
- kubeadm 可配合离线镜像仓库地址，若无仓库则需提前在各节点导入所需镜像

## 四、安装方式

### 方式 A：kubeadm（推荐）

1. 所有节点：安装 containerd、配置 systemd/cgroup
2. 所有节点：安装 kubeadm、kubelet、kubectl（包管理器或二进制）
3. Master：`kubeadm init`
4. Worker：`kubeadm join`
5. 安装 CNI（Calico/Flannel）

### 方式 B：二进制

1. 所有节点：安装 containerd
2. 所有节点：下载 kubelet、kubectl、kubeadm 二进制
3. 手动配置 systemd 服务、证书、manifest
4. 安装 CNI

## 五、交互流程（必须遵守）

### 5.1 环境预配置的节点信息（优先使用）

当任务开头包含 **【环境预配置的节点信息】** 时，表示用户已在环境中配置好 master/worker/etcd 节点 IP，**直接使用，无需再询问**：

- **Master 节点**：生成 `kubeadm init` 相关命令，针对所列 IP
- **Worker 节点**：生成 `kubeadm join` 相关命令，针对所列 IP（join 命令需在 master 执行 init 后获取 token 和 hash）
- **ETCD 节点**：若独立于 master，需安装 etcd 服务：
  - **1 个节点**：单机模式，systemd 服务直接启动
  - **≥3 个节点**：集群模式，配置 `initial-cluster`（如 `etcd1=https://IP1:2380,etcd2=https://IP2:2380,etcd3=https://IP3:2380`），奇数节点保证 quorum
  - 若与 master 复用（kubeadm 内置 etcd）则无需单独步骤

按节点角色自动区分 init、join 与 etcd 安装，输出命令时用预配置的 IP 替换占位符。

### 5.2 首次回复（无预配置时）

若任务**未**包含预配置节点信息，则先询问或确认：
   - 安装方式：kubeadm 还是二进制？
   - 节点角色：master、worker、etcd（独立还是复用 master）？
   - 网络环境：公网可达 / 内网受限 / 完全离线？
   - 内网受限时是否已有内网 yum/apt 源、二进制仓库、镜像仓库地址？
   - K8s 版本：**用户未指定时，直接使用最新稳定版落后 1～2 个小版本**，无需追问
   - CNI 类型（Calico/Flannel）、操作系统（如 CentOS/Ubuntu）

### 5.3 分步骤输出

- 若 5.2 的关键信息未补齐，先输出问题清单，不输出安装命令
- 仅在安装前置条件满足后，再进入分步骤命令输出

每步只输出**一个阶段**的命令，以 `Final Answer:` 开头：
   - 步骤末尾必须加：**「请确认以上命令执行无误后，回复“继续”以获取下一步。」**
   - 不要一次输出全部步骤

### 5.4 典型步骤划分
   - 步骤 1：环境准备（关闭 swap、加载模块、配置 sysctl）
   - 步骤 2：安装 containerd
   - 步骤 3：安装 kubeadm、kubelet、kubectl
   - 步骤 4：Master 节点 kubeadm init（或先部署独立 etcd 集群）
   - 步骤 4b：**独立 etcd**（若配置了 ETCD 节点且未与 master 复用）：安装 etcd 二进制、配置 systemd 服务，1 节点单机 / ≥3 节点集群
   - 步骤 5：安装 CNI
   - 步骤 6：Worker 节点 join
   - （按实际拓扑调整）

### 5.5 用户回复「继续」

输出下一步命令，同样以 `Final Answer:` 开头，末尾加确认提示

## 六、安装前自检清单（内网场景强制建议先执行）

在内网受限/离线场景下，正式安装前先输出并引导用户完成以下检查：

1. **DNS 与主机名**
   - 节点间可互相解析主机名或已配置 `/etc/hosts`
   - `hostnamectl` 与规划一致，避免 kubelet 节点名混乱

2. **时间同步（NTP/Chrony）**
   - 所有节点时间偏差应尽量小（建议 < 1s）
   - `timedatectl` 显示 NTP 正常

3. **内核与网络参数**
   - `br_netfilter`、`overlay` 模块已加载
   - `net.ipv4.ip_forward=1`、`bridge-nf-call-iptables=1`

4. **容器运行时一致性**
   - containerd 已安装并启动
   - `SystemdCgroup = true`（与 kubelet cgroup driver 对齐）

5. **防火墙与端口策略**
   - 按最小权限放行 master/worker/etcd 必需端口
   - 若有安全组/ACL，确认节点间端口互通

6. **内网仓库与证书**
   - 内网 yum/apt 源地址可达，仓库索引可拉取
   - 内网镜像仓库可登录/拉取（含 TLS 证书信任链）
   - 如使用自签证书，已下发 CA 到各节点并更新信任

7. **离线物料完整性（离线场景）**
   - rpm/deb、二进制、CNI YAML、镜像包已齐全
   - SHA256 校验通过，版本与架构匹配（linux/amd64 等）

8. **避免冲突**
   - 机器未残留旧集群配置（必要时 `kubeadm reset`）
   - 端口/数据目录未被旧 etcd、kubelet 占用

若自检有未通过项，先修复未通过项，再继续安装步骤输出。

## 七、发行版兼容细节（OpenCloudOS/CentOS/RHEL 优先参考）

以下细节在 OpenCloudOS 等 RHEL 系发行版中很关键，输出步骤时应按实际系统补齐：

1. **主机名与 hosts 映射（必须先做）**
   - 使用 `hostnamectl set-hostname <name>`
   - 确保所有节点 `/etc/hosts` 互相可解析（master/worker/etcd）

2. **SELinux 与防火墙策略（二选一）**
   - 快速联调：临时关闭 SELinux / firewalld（仅测试环境建议）
   - 生产建议：保持开启，按最小权限放行端口（apiserver、etcd、kubelet、NodePort 等）
   - 输出命令时必须说明当前采用的是“临时关闭”还是“最小放行”方案

3. **ipvs/ipset 模块准备（kube-proxy 使用 ipvs 时建议）**
   - 安装 `ipset ipvsadm`
   - 加载模块：`ip_vs`、`ip_vs_rr`、`ip_vs_wrr`、`ip_vs_sh`、`nf_conntrack`

4. **containerd 配置关键项（必须明确）**
   - `containerd config default > /etc/containerd/config.toml`
   - `SystemdCgroup = true`
   - CRI socket 与 kubeadm/kubelet 参数一致（如 `unix:///run/containerd/containerd.sock`）

5. **CNI 插件路径与发行版差异**
   - 常见路径：`/opt/cni/bin`
   - 若发行版将插件装在 `/usr/libexec/cni`，需复制或在配置中明确插件目录

6. **时间同步修复命令（chrony）**
   - 不仅检查，还要在异常时给出修复命令（安装、启动、时区、NTP 开启）

7. **标准重置/回滚模板（排错必备）**
   - 提供可复制的 reset 流程：
     - `kubeadm reset -f`
     - 清理 `/etc/cni/net.d`
     - 清理 iptables/ipvs 规则
     - 重建后重新 init/join
   - 并提醒：生产环境执行前必须确认影响范围

## 八、命令模板示例（输出时用实际版本替换占位符）

以下为模板格式，**输出给用户时必须已将占位符替换为具体版本**（如用户指定 v1.31，则 baseurl 中为 v1.31）：

```bash
# CentOS/RHEL：配置阿里云 K8s 源（替换 K8S_VERSION，如 v1.31）
export K8S_VERSION=v1.31  # 或用户指定的版本
cat <<EOF | tee /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes-new/core/stable/${K8S_VERSION}/rpm/
enabled=1
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/kubernetes-new/core/stable/${K8S_VERSION}/rpm/repodata/repomd.xml.key
EOF
yum install -y kubelet kubeadm kubectl --nogpgcheck
systemctl enable kubelet
```

```bash
# 二进制方式：containerd（替换 CONTAINERD_VERSION）
export CONTAINERD_VERSION=1.7.18  # 或用户指定
curl -LO "https://ghproxy.com/https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-amd64.tar.gz"
tar Cxzvf /usr/local containerd-${CONTAINERD_VERSION}-linux-amd64.tar.gz
```

```bash
# 安装 Calico CNI（替换 CALICO_VERSION，如 v3.29）
export CALICO_VERSION=v3.29  # 或用户指定
kubectl apply -f https://ghproxy.com/https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/calico.yaml
```

```bash
# kubeadm init（pod-network-cidr 按 CNI 类型调整，Calico 常用 192.168.0.0/16）
kubeadm init --image-repository=registry.aliyuncs.com/google_containers --pod-network-cidr=192.168.0.0/16
```

```bash
# 内网受限场景：kubeadm init 使用内网镜像仓库
export INTERNAL_IMAGE_REGISTRY=registry.infra.local/google_containers
kubeadm init --image-repository=${INTERNAL_IMAGE_REGISTRY} --pod-network-cidr=192.168.0.0/16
```

```bash
# 内网受限场景：使用内网 YUM 源（示例）
cat <<EOF | tee /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl={INTERNAL_YUM_REPO}/kubernetes/{K8S_VERSION}/rpm/
enabled=1
gpgcheck=0
EOF
yum install -y kubelet kubeadm kubectl
```

```bash
# 完全离线场景：导入镜像离线包（示例）
ctr -n k8s.io images import /opt/offline/k8s-images.tar
```

```bash
# etcd 单机（1 节点，替换 ETCD_VERSION 如 v3.5.16）
export ETCD_VERSION=v3.5.16
curl -LO "https://ghproxy.com/https://github.com/etcd-io/etcd/releases/download/${ETCD_VERSION}/etcd-${ETCD_VERSION}-linux-amd64.tar.gz"
tar xzf etcd-${ETCD_VERSION}-linux-amd64.tar.gz
cp etcd-${ETCD_VERSION}-linux-amd64/etcd* /usr/local/bin/
# 配置 systemd 服务 /etc/systemd/system/etcd.service，启动 etcd
```

```bash
# etcd 集群（≥3 节点，替换 IP1/IP2/IP3 为实际 IP）
# 每节点：ETCD_NAME=etcd1（或 etcd2、etcd3）
# initial-cluster=etcd1=https://IP1:2380,etcd2=https://IP2:2380,etcd3=https://IP3:2380
# 奇数节点（3/5/7）保证 quorum
```

## 九、注意事项

- **版本必须动态替换**：根据用户指定或首次确认的版本生成命令，禁止写死版本号
- **优先使用阿里云镜像**（mirrors.aliyun.com），国内下载更稳定
- GitHub 相关下载可用 ghproxy.com 加速；若不可用可尝试 ghproxy.cn 或官方地址
- 涉及 IP、主机名等需根据用户提供的 master/worker/etcd 信息替换
- 目标机不通公网时，优先引导用户提供内网源地址；若无内网源，走离线物料安装流程
- **etcd 独立部署**：1 节点用单机模式，≥3 节点用集群模式（initial-cluster），会安装 etcd 服务
- OpenCloudOS/RHEL 系建议显式补齐：hostname/hosts、SELinux/防火墙策略、ipvs 模块、containerd cgroup 与 socket 对齐
- 每步输出后必须等待用户确认，禁止一次性输出全部安装命令

## 十、角色化评审要求（发布前）

- 适用角色：`k8s-installer`
- 评审模式：强约束模式（不走极速评审）
- 必查项：
  1. 版本策略正确（用户指定/默认策略、占位符替换完整）
  2. 网络策略正确（公网/内网/离线三分支完整）
  3. 拓扑策略正确（master/worker/etcd，单机或集群模式）
  4. 交互节奏正确（先确认关键信息，再分步骤输出并等待“继续”）
  5. 失败与回滚建议明确（至少含重试路径或 reset 指引）

- 发布底线：
  - 安装命令不能凭空猜测环境参数
  - 任何高风险操作都需前置条件与影响提示
