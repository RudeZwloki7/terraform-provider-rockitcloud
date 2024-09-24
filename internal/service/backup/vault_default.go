package backup

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/backup"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
)

func ResourceDefaultVault() *schema.Resource {
	return &schema.Resource{
		Create: resourceDefaultVaultCreate,
		Read:   resourceDefaultVaultRead,
		Delete: resourceDefaultVaultDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"recovery_points": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func resourceDefaultVaultCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).BackupConn

	input := &backup.ListBackupVaultsInput{}
	output, err := conn.ListBackupVaults(input)
	if err != nil {
		return fmt.Errorf("error listing Backup Vaults: %w", err)
	}

	if len(output.BackupVaultList) == 0 {
		name := "Default"
		input := &backup.CreateBackupVaultInput{
			BackupVaultName: aws.String(name),
		}

		if _, err = conn.CreateBackupVault(input); err != nil {
			return fmt.Errorf("error creating Backup Vault (%s): %w", name, err)
		}
		d.SetId(name)

		return resourceDefaultVaultRead(d, meta)
	}

	vault := output.BackupVaultList[0]
	log.Printf("[INFO] Found existing Backup Vault (%s)", aws.StringValue(vault.BackupVaultName))

	d.SetId(aws.StringValue(vault.BackupVaultName))

	return resourceDefaultVaultRead(d, meta)
}

func resourceDefaultVaultRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).BackupConn

	output, err := FindBackupVaultByName(conn, d.Id())

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, errCodeVaultNotFound) {
		log.Printf("[WARN] Backup Vault (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("error reading Backup Vault (%s): %w", d.Id(), err)
	}

	d.Set("arn", output.BackupVaultArn)
	d.Set("name", output.BackupVaultName)
	d.Set("recovery_points", output.NumberOfRecoveryPoints)

	return nil
}

func resourceDefaultVaultDelete(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[WARN] Default Backup Vault (%s) not deleted, removing from state", d.Id())

	return nil
}
