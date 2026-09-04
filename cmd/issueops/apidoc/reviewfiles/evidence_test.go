package reviewfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEvidenceRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func TestEvidenceDirectServiceThrows(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/users/users.controller.ts": `import { Controller, Get } from '@nestjs/common';
@Controller('users')
export class UsersController {
  constructor(private readonly usersService: UsersService) {}

  @Get(':id')
  findOne(@Param('id') id: string) {
    return this.usersService.findOne(id);
  }
}
`,
		"src/users/users.service.ts": `import { Injectable, NotFoundException } from '@nestjs/common';

@Injectable()
export class UsersService {
  async findOne(id: string) {
    const user = await this.repo.findById(id);
    if (!user) {
      throw new NotFoundException('user not found');
    }
    return user;
  }
}
`,
	})
	evidence := Evidence(dir, []string{"src/users/users.controller.ts"})
	for _, want := range []string{"users.service.ts", "NotFoundException", "findOne"} {
		if !strings.Contains(evidence, want) {
			t.Fatalf("evidence missing %q:\n%s", want, evidence)
		}
	}
}

func TestEvidenceMicroserviceStringPatternHop(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"gateway/orders.controller.ts": `import { Controller, Post } from '@nestjs/common';
import { ClientProxy } from '@nestjs/microservices';

@Controller('orders')
export class OrdersController {
  constructor(private readonly client: ClientProxy) {}

  @Post()
  create(@Body() dto: any) {
    return this.client.send('orders_create', dto).toPromise();
  }
}
`,
		"remote/orders.controller.ts": `import { Controller } from '@nestjs/common';
import { MessagePattern, Payload } from '@nestjs/microservices';

@Controller()
export class OrdersController {
  constructor(private readonly ordersService: OrdersService) {}

  @MessagePattern('orders_create')
  create(@Payload() dto: any) {
    return this.ordersService.create(dto);
  }
}
`,
		"remote/orders.service.ts": `import { Injectable } from '@nestjs/common';
import { RpcException } from '@nestjs/microservices';

@Injectable()
export class OrdersService {
  async create(dto: any) {
    if (dto.productId == null) {
      throw new RpcException({ status: 'CONFLICT', message: 'duplicate order' });
    }
    return dto;
  }
}
`,
	})
	evidence := Evidence(dir, []string{"gateway/orders.controller.ts"})
	for _, want := range []string{"orders_create", "RpcException", "CONFLICT", "orders.service.ts"} {
		if !strings.Contains(evidence, want) {
			t.Fatalf("evidence missing %q:\n%s", want, evidence)
		}
	}
	if strings.Contains(evidence, "remote/orders.controller.ts#") {
		t.Fatalf("remote message handler should not be treated as an HTTP controller under review:\n%s", evidence)
	}
}

func TestEvidenceCleanRepoIsEmpty(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/health.controller.ts": `import { Controller, Get } from '@nestjs/common';

@Controller('health')
export class HealthController {
  @Get()
  check() {
    return { ok: true };
  }
}
`,
	})
	if evidence := Evidence(dir, []string{"src/health.controller.ts"}); strings.TrimSpace(evidence) != "" {
		t.Fatalf("expected no evidence, got:\n%s", evidence)
	}
}

func TestEvidenceSkipsNodeModules(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/app.controller.ts": `import { Controller, Get } from '@nestjs/common';

@Controller('app')
export class AppController {
  constructor(private readonly appService: AppService) {}

  @Get()
  hello() {
    return this.appService.hello();
  }
}
`,
		"node_modules/vendor/index.ts": `export class AppService {
  hello() {
    throw new Error('vendor noise');
  }
}
`,
	})
	evidence := Evidence(dir, []string{"src/app.controller.ts"})
	if strings.Contains(evidence, "vendor") {
		t.Fatalf("node_modules leaked into evidence:\n%s", evidence)
	}
}
