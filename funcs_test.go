package main

import (
	"io/ioutil"
	"log"
	"math"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

const testSecret = "/mschurenko/entrypoint/test_secret"
const testSecretValue = "mysecret"
const s3Bucket = "mschurenko-test"
const s3Key = "fixtures/vars.yml"

func TestMain(m *testing.M) {
	r := ec2Metadata("region")
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(r)}))
	setup(sess)
	rc := m.Run()
	teardown(sess)
	os.Exit(rc)
}

func describeSecret(svc *secretsmanager.SecretsManager) (*secretsmanager.DescribeSecretOutput, error) {
	input := &secretsmanager.DescribeSecretInput{SecretId: aws.String(testSecret)}
	return svc.DescribeSecret(input)
}

func secretExists(svc *secretsmanager.SecretsManager) bool {
	o, err := describeSecret(svc)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
				return false
			}
		}
		log.Fatalf("desribe secret failed: %v", err)
	}

	if o.DeletedDate == nil {
		return true
	}

	// secret exists but is still not deleted, so wait until secret is gone
	for i := 0; i < 10; i++ {
		if _, err := describeSecret(svc); err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
					break
				}
			}
		}
		time.Sleep(time.Duration(math.Exp2(float64(i))) * time.Second)
	}

	return false
}

func createSecret(svc *secretsmanager.SecretsManager) error {
	input := &secretsmanager.CreateSecretInput{
		Name:         aws.String(testSecret),
		SecretString: aws.String(testSecretValue),
	}
	_, err := svc.CreateSecret(input)
	return err
}

func deleteSecret(svc *secretsmanager.SecretsManager) error {
	input := &secretsmanager.DeleteSecretInput{
		ForceDeleteWithoutRecovery: aws.Bool(true),
		SecretId:                   aws.String(testSecret),
	}
	_, err := svc.DeleteSecret(input)
	return err
}

func updateSecret(svc *secretsmanager.SecretsManager) error {
	input := &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(testSecret),
		SecretString: aws.String(testSecretValue),
	}
	_, err := svc.UpdateSecret(input)
	return err
}

func setup(sess *session.Session) {
	smSvc := secretsmanager.New(sess)
	switch secretExists(smSvc) {
	case true:
		if err := updateSecret(smSvc); err != nil {
			log.Fatalf("update secret failed: %v", err)
		}
	default:
		if err := createSecret(smSvc); err != nil {
			log.Fatalf("create secret failed: %v", err)
		}
		for i := 0; i < 10; i++ {
			if !secretExists(smSvc) {
				time.Sleep(time.Duration(math.Exp2(float64(i))) * time.Second)
			} else {
				break
			}
		}
	}

}

func teardown(sess *session.Session) {
	svc := secretsmanager.New(sess)
	if err := deleteSecret(svc); err != nil {
		log.Fatalf("could not delete secret: %v\n", err)
	}
}

func TestCheckEntrypointVarValid(t *testing.T) {
	valid := "ENTRYPOINT_VARS_FILE"

	if !checkEntrypointVar(valid) {
		t.Errorf("%v should be a valid env var", valid)
	}

}

func TestCheckEntrypointVarInValid(t *testing.T) {
	invalid := "ENTRYPOINT_X"

	if checkEntrypointVar(invalid) {
		t.Errorf("%v should not be a valid env var", invalid)
	}

}

func TestRenderStr(t *testing.T) {
	tmpl := `{{ mul 2 2 }}`
	exepcted := `4`
	resp := newTpl("test").renderStr(tmpl)
	if resp != exepcted {
		t.Errorf("%v is not equal to %v\n", resp, exepcted)
	}

}

func TestRenderTmpl(t *testing.T) {
	tmplName := "test.conf.tmpl"

	exceptedStr := `
	MY_ENV is testing
	value of /mschurenko/entrypoint/test_secret is mysecret
	aws region is us-west-2
	`

	tmplStr := `
	MY_ENV is {{ env "MY_ENV" }}
	value of /mschurenko/entrypoint/test_secret is {{ secret "/mschurenko/entrypoint/test_secret" }}
	aws region is {{ ec2Metadata "region" }}
	`

	if err := ioutil.WriteFile(tmplName, []byte(tmplStr), 0644); err != nil {
		t.Error(err)
	}
	defer os.Remove(tmplName)

	tpl := newTpl(tmplName)
	tpl.renderFile()
	defer os.Remove(tpl.output)

	sb, err := ioutil.ReadFile(tpl.output)
	if err != nil {
		t.Error(err)
	}

	if string(sb) != exceptedStr {
		t.Errorf("%v is not equal to %v", string(sb), exceptedStr)
	}

}
